package tusd

import (
	"encoding/base64"
	"errors"
	"io"
	"log"
	"math"
	"net"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

const UploadLengthDeferred = "1"

var (
	reExtractFileID  = regexp.MustCompile(`([^/]+)\/?$`)
	reForwardedHost  = regexp.MustCompile(`host=([^,]+)`)
	reForwardedProto = regexp.MustCompile(`proto=(https?)`)
	reMimeType       = regexp.MustCompile(`^[a-z]+\/[a-z\-\+]+$`)
)

// HTTPError represents an error with an additional status code attached
// which may be used when this error is sent in a HTTP response.
// See the net/http package for standardized status codes.
type HTTPError interface {
	error
	StatusCode() int
	Body() []byte
}

type httpError struct {
	error
	statusCode int
}

func (err httpError) StatusCode() int {
	return err.statusCode
}

func (err httpError) Body() []byte {
	return []byte(err.Error())
}

// NewHTTPError adds the given status code to the provided error and returns
// the new error instance. The status code may be used in corresponding HTTP
// responses. See the net/http package for standardized status codes.
func NewHTTPError(err error, statusCode int) HTTPError {
	return httpError{err, statusCode}
}

var (
	ErrUnsupportedVersion               = NewHTTPError(errors.New("unsupported version"), http.StatusPreconditionFailed)
	ErrMaxSizeExceeded                  = NewHTTPError(errors.New("maximum size exceeded"), http.StatusRequestEntityTooLarge)
	ErrInvalidContentType               = NewHTTPError(errors.New("missing or invalid Content-Type header"), http.StatusBadRequest)
	ErrInvalidUploadLength              = NewHTTPError(errors.New("missing or invalid Upload-Length header"), http.StatusBadRequest)
	ErrInvalidOffset                    = NewHTTPError(errors.New("missing or invalid Upload-Offset header"), http.StatusBadRequest)
	ErrNotFound                         = NewHTTPError(errors.New("upload not found"), http.StatusNotFound)
	ErrFileLocked                       = NewHTTPError(errors.New("file currently locked"), 423) // Locked (WebDAV) (RFC 4918)
	ErrMismatchOffset                   = NewHTTPError(errors.New("mismatched offset"), http.StatusConflict)
	ErrSizeExceeded                     = NewHTTPError(errors.New("resource's size exceeded"), http.StatusRequestEntityTooLarge)
	ErrNotImplemented                   = NewHTTPError(errors.New("feature not implemented"), http.StatusNotImplemented)
	ErrUploadNotFinished                = NewHTTPError(errors.New("one of the partial uploads is not finished"), http.StatusBadRequest)
	ErrInvalidConcat                    = NewHTTPError(errors.New("invalid Upload-Concat header"), http.StatusBadRequest)
	ErrModifyFinal                      = NewHTTPError(errors.New("modifying a final upload is not allowed"), http.StatusForbidden)
	ErrUploadLengthAndUploadDeferLength = NewHTTPError(errors.New("provided both Upload-Length and Upload-Defer-Length"), http.StatusBadRequest)
	ErrInvalidUploadDeferLength         = NewHTTPError(errors.New("invalid Upload-Defer-Length header"), http.StatusBadRequest)
)

// UnroutedHandler exposes methods to handle requests as part of the tus protocol,
// such as PostFile, HeadFile, PatchFile and DelFile. In addition the GetFile method
// is provided which is, however, not part of the specification.
type UnroutedHandler struct {
	config        Config
	composer      *StoreComposer
	isBasePathAbs bool
	basePath      string
	logger        *log.Logger
	extensions    string

	// CompleteUploads is used to send notifications whenever an upload is
	// completed by a user. The FileInfo will contain information about this
	// upload after it is completed. Sending to this channel will only
	// happen if the NotifyCompleteUploads field is set to true in the Config
	// structure. Notifications will also be sent for completions using the
	// Concatenation extension.
	CompleteUploads chan FileInfo
	// TerminatedUploads is used to send notifications whenever an upload is
	// terminated by a user. The FileInfo will contain information about this
	// upload gathered before the termination. Sending to this channel will only
	// happen if the NotifyTerminatedUploads field is set to true in the Config
	// structure.
	TerminatedUploads chan FileInfo
	// UploadProgress is used to send notifications about the progress of the
	// currently running uploads. For each open PATCH request, every second
	// a FileInfo instance will be send over this channel with the Offset field
	// being set to the number of bytes which have been transfered to the server.
	// Please be aware that this number may be higher than the number of bytes
	// which have been stored by the data store! Sending to this channel will only
	// happen if the NotifyUploadProgress field is set to true in the Config
	// structure.
	UploadProgress chan FileInfo
	// CreatedUploads is used to send notifications about the uploads having been
	// created. It triggers post creation and therefore has all the FileInfo incl.
	// the ID available already. It facilitates the post-create hook. Sending to
	// this channel will only happen if the NotifyCreatedUploads field is set to
	// true in the Config structure.
	CreatedUploads chan FileInfo
	// Metrics provides numbers of the usage for this handler.
	Metrics Metrics
}

// NewUnroutedHandler creates a new handler without routing using the given
// configuration. It exposes the http handlers which need to be combined with
// a router (aka mux) of your choice. If you are looking for preconfigured
// handler see NewHandler.
func NewUnroutedHandler(config Config) (*UnroutedHandler, error) {
	if err := config.validate(); err != nil {
		return nil, err
	}

	// Only promote extesions using the Tus-Extension header which are implemented
	extensions := "creation,creation-with-upload"
	if config.StoreComposer.UsesTerminater {
		extensions += ",termination"
	}
	if config.StoreComposer.UsesConcater {
		extensions += ",concatenation"
	}
	if config.StoreComposer.UsesLengthDeferrer {
		extensions += ",creation-defer-length"
	}

	handler := &UnroutedHandler{
		config:            config,
		composer:          config.StoreComposer,
		basePath:          config.BasePath,
		isBasePathAbs:     config.isAbs,
		CompleteUploads:   make(chan FileInfo),
		TerminatedUploads: make(chan FileInfo),
		UploadProgress:    make(chan FileInfo),
		CreatedUploads:    make(chan FileInfo),
		logger:            config.Logger,
		extensions:        extensions,
		Metrics:           newMetrics(),
	}

	return handler, nil
}

// Middleware checks various aspects of the request and ensures that it
// conforms with the spec. Also handles method overriding for clients which
// cannot make PATCH AND DELETE requests. If you are using the tusd handlers
// directly you will need to wrap at least the POST and PATCH endpoints in
// this middleware.
func (handler *UnroutedHandler) Middleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Allow overriding the HTTP method. The reason for this is
		// that some libraries/environments to not support PATCH and
		// DELETE requests, e.g. Flash in a browser and parts of Java
		if newMethod := r.Header.Get("X-HTTP-Method-Override"); newMethod != "" {
			r.Method = newMethod
		}

		handler.log("RequestIncoming", "method", r.Method, "path", r.URL.Path)

		go handler.Metrics.incRequestsTotal(r.Method)

		header := w.Header()

		if origin := r.Header.Get("Origin"); origin != "" {
			header.Set("Access-Control-Allow-Origin", origin)

			if r.Method == "OPTIONS" {
				// Preflight request
				header.Add("Access-Control-Allow-Methods", "POST, GET, HEAD, PATCH, DELETE, OPTIONS")
				header.Add("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Upload-Length, Upload-Offset, Tus-Resumable, Upload-Metadata, Upload-Defer-Length, Upload-Concat")
				header.Set("Access-Control-Max-Age", "86400")

			} else {
				// Actual request
				header.Add("Access-Control-Expose-Headers", "Upload-Offset, Location, Upload-Length, Tus-Version, Tus-Resumable, Tus-Max-Size, Tus-Extension, Upload-Metadata, Upload-Defer-Length, Upload-Concat")
			}
		}

		// Set current version used by the server
		header.Set("Tus-Resumable", "1.0.0")

		// Add nosniff to all responses https://golang.org/src/net/http/server.go#L1429
		header.Set("X-Content-Type-Options", "nosniff")

		// Set appropriated headers in case of OPTIONS method allowing protocol
		// discovery and end with an 204 No Content
		if r.Method == "OPTIONS" {
			if handler.config.MaxSize > 0 {
				header.Set("Tus-Max-Size", strconv.FormatInt(handler.config.MaxSize, 10))
			}

			header.Set("Tus-Version", "1.0.0")
			header.Set("Tus-Extension", handler.extensions)

			// Although the 204 No Content status code is a better fit in this case,
			// since we do not have a response body included, we cannot use it here
			// as some browsers only accept 200 OK as successful response to a
			// preflight request. If we send them the 204 No Content the response
			// will be ignored or interpreted as a rejection.
			// For example, the Presto engine, which is used in older versions of
			// Opera, Opera Mobile and Opera Mini, handles CORS this way.
			handler.sendResp(w, r, http.StatusOK)
			return
		}

		// Test if the version sent by the client is supported
		// GET methods are not checked since a browser may visit this URL and does
		// not include this header. This request is not part of the specification.
		if r.Method != "GET" && r.Header.Get("Tus-Resumable") != "1.0.0" {
			handler.sendError(w, r, ErrUnsupportedVersion)
			return
		}

		// Proceed with routing the request
		h.ServeHTTP(w, r)
	})
}

// PostFile creates a new file upload using the datastore after validating the
// length and parsing the metadata.
func (handler *UnroutedHandler) PostFile(w http.ResponseWriter, r *http.Request) {
	// Check for presence of application/offset+octet-stream. If another content
	// type is defined, it will be ignored and treated as none was set because
	// some HTTP clients may enforce a default value for this header.
	containsChunk := r.Header.Get("Content-Type") == "application/offset+octet-stream"

	// Only use the proper Upload-Concat header if the concatenation extension
	// is even supported by the data store.
	var concatHeader string
	if handler.composer.UsesConcater {
		concatHeader = r.Header.Get("Upload-Concat")
	}

	// Parse Upload-Concat header
	isPartial, isFinal, partialUploads, err := parseConcat(concatHeader)
	if err != nil {
		handler.sendError(w, r, err)
		return
	}

	// If the upload is a final upload created by concatenation multiple partial
	// uploads the size is sum of all sizes of these files (no need for
	// Upload-Length header)
	var size int64
	var sizeIsDeferred bool
	if isFinal {
		// A final upload must not contain a chunk within the creation request
		if containsChunk {
			handler.sendError(w, r, ErrModifyFinal)
			return
		}

		size, err = handler.sizeOfUploads(partialUploads)
		if err != nil {
			handler.sendError(w, r, err)
			return
		}
	} else {
		uploadLengthHeader := r.Header.Get("Upload-Length")
		uploadDeferLengthHeader := r.Header.Get("Upload-Defer-Length")
		size, sizeIsDeferred, err = handler.validateNewUploadLengthHeaders(uploadLengthHeader, uploadDeferLengthHeader)
		if err != nil {
			handler.sendError(w, r, err)
			return
		}
	}

	// Test whether the size is still allowed
	if handler.config.MaxSize > 0 && size > handler.config.MaxSize {
		handler.sendError(w, r, ErrMaxSizeExceeded)
		return
	}

	// Parse metadata
	meta := ParseMetadataHeader(r.Header.Get("Upload-Metadata"))

	info := FileInfo{
		Size:           size,
		SizeIsDeferred: sizeIsDeferred,
		MetaData:       meta,
		IsPartial:      isPartial,
		IsFinal:        isFinal,
		PartialUploads: partialUploads,
	}

	id, err := handler.composer.Core.NewUpload(info)
	if err != nil {
		handler.sendError(w, r, err)
		return
	}

	info.ID = id

	// Add the Location header directly after creating the new resource to even
	// include it in cases of failure when an error is returned
	url := handler.absFileURL(r, id)
	w.Header().Set("Location", url)

	go handler.Metrics.incUploadsCreated()
	handler.log("UploadCreated", "id", id, "size", i64toa(size), "url", url)

	if handler.config.NotifyCreatedUploads {
		handler.CreatedUploads <- info
	}

	if isFinal {
		if err := handler.composer.Concater.ConcatUploads(id, partialUploads); err != nil {
			handler.sendError(w, r, err)
			return
		}
		info.Offset = size

		if handler.config.NotifyCompleteUploads {
			handler.CompleteUploads <- info
		}
	}

	if containsChunk {
		if handler.composer.UsesLocker {
			locker := handler.composer.Locker
			if err := locker.LockUpload(id); err != nil {
				handler.sendError(w, r, err)
				return
			}

			defer locker.UnlockUpload(id)
		}

		if err := handler.writeChunk(id, info, w, r); err != nil {
			handler.sendError(w, r, err)
			return
		}
	} else if !sizeIsDeferred && size == 0 {
		// Directly finish the upload if the upload is empty (i.e. has a size of 0).
		// This statement is in an else-if block to avoid causing duplicate calls
		// to finishUploadIfComplete if an upload is empty and contains a chunk.
		handler.finishUploadIfComplete(info)
	}

	handler.sendResp(w, r, http.StatusCreated)
}

// HeadFile returns the length and offset for the HEAD request
func (handler *UnroutedHandler) HeadFile(w http.ResponseWriter, r *http.Request) {

	id, err := extractIDFromPath(r.URL.Path)
	if err != nil {
		handler.sendError(w, r, err)
		return
	}

	if handler.composer.UsesLocker {
		locker := handler.composer.Locker
		if err := locker.LockUpload(id); err != nil {
			handler.sendError(w, r, err)
			return
		}

		defer locker.UnlockUpload(id)
	}

	info, err := handler.composer.Core.GetInfo(id)
	if err != nil {
		handler.sendError(w, r, err)
		return
	}

	// Add Upload-Concat header if possible
	if info.IsPartial {
		w.Header().Set("Upload-Concat", "partial")
	}

	if info.IsFinal {
		v := "final;"
		for _, uploadID := range info.PartialUploads {
			v += handler.absFileURL(r, uploadID) + " "
		}
		// Remove trailing space
		v = v[:len(v)-1]

		w.Header().Set("Upload-Concat", v)
	}

	if len(info.MetaData) != 0 {
		w.Header().Set("Upload-Metadata", SerializeMetadataHeader(info.MetaData))
	}

	if info.SizeIsDeferred {
		w.Header().Set("Upload-Defer-Length", UploadLengthDeferred)
	} else {
		w.Header().Set("Upload-Length", strconv.FormatInt(info.Size, 10))
	}

	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Upload-Offset", strconv.FormatInt(info.Offset, 10))
	handler.sendResp(w, r, http.StatusOK)
}

// PatchFile adds a chunk to an upload. This operation is only allowed
// if enough space in the upload is left.
func (handler *UnroutedHandler) PatchFile(w http.ResponseWriter, r *http.Request) {

	// Check for presence of application/offset+octet-stream
	if r.Header.Get("Content-Type") != "application/offset+octet-stream" {
		handler.sendError(w, r, ErrInvalidContentType)
		return
	}

	// Check for presence of a valid Upload-Offset Header
	offset, err := strconv.ParseInt(r.Header.Get("Upload-Offset"), 10, 64)
	if err != nil || offset < 0 {
		handler.sendError(w, r, ErrInvalidOffset)
		return
	}

	id, err := extractIDFromPath(r.URL.Path)
	if err != nil {
		handler.sendError(w, r, err)
		return
	}

	if handler.composer.UsesLocker {
		locker := handler.composer.Locker
		if err := locker.LockUpload(id); err != nil {
			handler.sendError(w, r, err)
			return
		}

		defer locker.UnlockUpload(id)
	}

	info, err := handler.composer.Core.GetInfo(id)
	if err != nil {
		handler.sendError(w, r, err)
		return
	}

	// Modifying a final upload is not allowed
	if info.IsFinal {
		handler.sendError(w, r, ErrModifyFinal)
		return
	}

	if offset != info.Offset {
		handler.sendError(w, r, ErrMismatchOffset)
		return
	}

	// Do not proxy the call to the data store if the upload is already completed
	if !info.SizeIsDeferred && info.Offset == info.Size {
		w.Header().Set("Upload-Offset", strconv.FormatInt(offset, 10))
		handler.sendResp(w, r, http.StatusNoContent)
		return
	}

	if r.Header.Get("Upload-Length") != "" {
		if !handler.composer.UsesLengthDeferrer {
			handler.sendError(w, r, ErrNotImplemented)
			return
		}
		if !info.SizeIsDeferred {
			handler.sendError(w, r, ErrInvalidUploadLength)
			return
		}
		uploadLength, err := strconv.ParseInt(r.Header.Get("Upload-Length"), 10, 64)
		if err != nil || uploadLength < 0 || uploadLength < info.Offset || (handler.config.MaxSize > 0 && uploadLength > handler.config.MaxSize) {
			handler.sendError(w, r, ErrInvalidUploadLength)
			return
		}
		if err := handler.composer.LengthDeferrer.DeclareLength(id, uploadLength); err != nil {
			handler.sendError(w, r, err)
			return
		}

		info.Size = uploadLength
		info.SizeIsDeferred = false
	}

	if err := handler.writeChunk(id, info, w, r); err != nil {
		handler.sendError(w, r, err)
		return
	}

	handler.sendResp(w, r, http.StatusNoContent)
}

// writeChunk reads the body from the requests r and appends it to the upload
// with the corresponding id. Afterwards, it will set the necessary response
// headers but will not send the response.
func (handler *UnroutedHandler) writeChunk(id string, info FileInfo, w http.ResponseWriter, r *http.Request) error {
	// Get Content-Length if possible
	length := r.ContentLength
	offset := info.Offset

	// Test if this upload fits into the file's size
	if !info.SizeIsDeferred && offset+length > info.Size {
		return ErrSizeExceeded
	}

	maxSize := info.Size - offset
	// If the upload's length is deferred and the PATCH request does not contain the Content-Length
	// header (which is allowed if 'Transfer-Encoding: chunked' is used), we still need to set limits for
	// the body size.
	if info.SizeIsDeferred {
		if handler.config.MaxSize > 0 {
			// Ensure that the upload does not exceed the maximum upload size
			maxSize = handler.config.MaxSize - offset
		} else {
			// If no upload limit is given, we allow arbitrary sizes
			maxSize = math.MaxInt64
		}
	}
	if length > 0 {
		maxSize = length
	}

	handler.log("ChunkWriteStart", "id", id, "maxSize", i64toa(maxSize), "offset", i64toa(offset))

	var bytesWritten int64
	// Prevent a nil pointer dereference when accessing the body which may not be
	// available in the case of a malicious request.
	if r.Body != nil {
		// Limit the data read from the request's body to the allowed maximum
		reader := io.LimitReader(r.Body, maxSize)

		if handler.config.NotifyUploadProgress {
			var stop chan<- struct{}
			reader, stop = handler.sendProgressMessages(info, reader)
			defer close(stop)
		}

		var err error
		bytesWritten, err = handler.composer.Core.WriteChunk(id, offset, reader)
		if err != nil {
			return err
		}
	}

	handler.log("ChunkWriteComplete", "id", id, "bytesWritten", i64toa(bytesWritten))

	// Send new offset to client
	newOffset := offset + bytesWritten
	w.Header().Set("Upload-Offset", strconv.FormatInt(newOffset, 10))
	go handler.Metrics.incBytesReceived(uint64(bytesWritten))
	info.Offset = newOffset

	return handler.finishUploadIfComplete(info)
}

// finishUploadIfComplete checks whether an upload is completed (i.e. upload offset
// matches upload size) and if so, it will call the data store's FinishUpload
// function and send the necessary message on the CompleteUpload channel.
func (handler *UnroutedHandler) finishUploadIfComplete(info FileInfo) error {
	// If the upload is completed, ...
	if !info.SizeIsDeferred && info.Offset == info.Size {
		// ... allow custom mechanism to finish and cleanup the upload
		if handler.composer.UsesFinisher {
			if err := handler.composer.Finisher.FinishUpload(info.ID); err != nil {
				return err
			}
		}

		// ... send the info out to the channel
		if handler.config.NotifyCompleteUploads {
			handler.CompleteUploads <- info
		}

		go handler.Metrics.incUploadsFinished()
	}

	return nil
}

// GetFile handles requests to download a file using a GET request. This is not
// part of the specification.
func (handler *UnroutedHandler) GetFile(w http.ResponseWriter, r *http.Request) {
	if !handler.composer.UsesGetReader {
		handler.sendError(w, r, ErrNotImplemented)
		return
	}

	id, err := extractIDFromPath(r.URL.Path)
	if err != nil {
		handler.sendError(w, r, err)
		return
	}

	if handler.composer.UsesLocker {
		locker := handler.composer.Locker
		if err := locker.LockUpload(id); err != nil {
			handler.sendError(w, r, err)
			return
		}

		defer locker.UnlockUpload(id)
	}

	info, err := handler.composer.Core.GetInfo(id)
	if err != nil {
		handler.sendError(w, r, err)
		return
	}

	// Set headers before sending responses
	w.Header().Set("Content-Length", strconv.FormatInt(info.Offset, 10))

	contentType, contentDisposition := filterContentType(info)
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", contentDisposition)

	// If no data has been uploaded yet, respond with an empty "204 No Content" status.
	if info.Offset == 0 {
		handler.sendResp(w, r, http.StatusNoContent)
		return
	}

	src, err := handler.composer.GetReader.GetReader(id)
	if err != nil {
		handler.sendError(w, r, err)
		return
	}

	handler.sendResp(w, r, http.StatusOK)
	io.Copy(w, src)

	// Try to close the reader if the io.Closer interface is implemented
	if closer, ok := src.(io.Closer); ok {
		closer.Close()
	}
}

// mimeInlineBrowserWhitelist is a map containing MIME types which should be
// allowed to be rendered by browser inline, instead of being forced to be
// downloadd. For example, HTML or SVG files are not allowed, since they may
// contain malicious JavaScript. In a similiar fashion PDF is not on this list
// as their parsers commonly contain vulnerabilities which can be exploited.
// The values of this map does not convei any meaning and are therefore just
// empty structs.
var mimeInlineBrowserWhitelist = map[string]struct{}{
	"text/plain": struct{}{},

	"image/png":  struct{}{},
	"image/jpeg": struct{}{},
	"image/gif":  struct{}{},
	"image/bmp":  struct{}{},
	"image/webp": struct{}{},

	"audio/wave":      struct{}{},
	"audio/wav":       struct{}{},
	"audio/x-wav":     struct{}{},
	"audio/x-pn-wav":  struct{}{},
	"audio/webm":      struct{}{},
	"video/webm":      struct{}{},
	"audio/ogg":       struct{}{},
	"video/ogg ":      struct{}{},
	"application/ogg": struct{}{},
}

// filterContentType returns the values for the Content-Type and
// Content-Disposition headers for a given upload. These values should be used
// in responses for GET requests to ensure that only non-malicious file types
// are shown directly in the browser. It will extract the file name and type
// from the "fileame" and "filetype".
// See https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Content-Disposition
func filterContentType(info FileInfo) (contentType string, contentDisposition string) {
	filetype := info.MetaData["filetype"]

	if reMimeType.MatchString(filetype) {
		// If the filetype from metadata is well formed, we forward use this
		// for the Content-Type header. However, only whitelisted mime types
		// will be allowed to be shown inline in the browser
		contentType = filetype
		if _, isWhitelisted := mimeInlineBrowserWhitelist[filetype]; isWhitelisted {
			contentDisposition = "inline"
		} else {
			contentDisposition = "attachment"
		}
	} else {
		// If the filetype from the metadata is not well formed, we use a
		// default type and force the browser to download the content.
		contentType = "application/octet-stream"
		contentDisposition = "attachment"
	}

	// Add a filename to Content-Disposition if one is available in the metadata
	if filename, ok := info.MetaData["filename"]; ok {
		contentDisposition += ";filename=" + strconv.Quote(filename)
	}

	return contentType, contentDisposition
}

// DelFile terminates an upload permanently.
func (handler *UnroutedHandler) DelFile(w http.ResponseWriter, r *http.Request) {
	// Abort the request handling if the required interface is not implemented
	if !handler.composer.UsesTerminater {
		handler.sendError(w, r, ErrNotImplemented)
		return
	}

	id, err := extractIDFromPath(r.URL.Path)
	if err != nil {
		handler.sendError(w, r, err)
		return
	}

	if handler.composer.UsesLocker {
		locker := handler.composer.Locker
		if err := locker.LockUpload(id); err != nil {
			handler.sendError(w, r, err)
			return
		}

		defer locker.UnlockUpload(id)
	}

	var info FileInfo
	if handler.config.NotifyTerminatedUploads {
		info, err = handler.composer.Core.GetInfo(id)
		if err != nil {
			handler.sendError(w, r, err)
			return
		}
	}

	err = handler.composer.Terminater.Terminate(id)
	if err != nil {
		handler.sendError(w, r, err)
		return
	}

	handler.sendResp(w, r, http.StatusNoContent)

	if handler.config.NotifyTerminatedUploads {
		handler.TerminatedUploads <- info
	}

	go handler.Metrics.incUploadsTerminated()
}

// Send the error in the response body. The status code will be looked up in
// ErrStatusCodes. If none is found 500 Internal Error will be used.
func (handler *UnroutedHandler) sendError(w http.ResponseWriter, r *http.Request, err error) {
	// Interpret os.ErrNotExist as 404 Not Found
	if os.IsNotExist(err) {
		err = ErrNotFound
	}

	// Errors for read timeouts contain too much information which is not
	// necessary for us and makes grouping for the metrics harder. The error
	// message looks like: read tcp 127.0.0.1:1080->127.0.0.1:53673: i/o timeout
	// Therefore, we use a common error message for all of them.
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		err = errors.New("read tcp: i/o timeout")
	}

	statusErr, ok := err.(HTTPError)
	if !ok {
		statusErr = NewHTTPError(err, http.StatusInternalServerError)
	}

	reason := append(statusErr.Body(), '\n')
	if r.Method == "HEAD" {
		reason = nil
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(len(reason)))
	w.WriteHeader(statusErr.StatusCode())
	w.Write(reason)

	handler.log("ResponseOutgoing", "status", strconv.Itoa(statusErr.StatusCode()), "method", r.Method, "path", r.URL.Path, "error", err.Error())

	go handler.Metrics.incErrorsTotal(statusErr)
}

// sendResp writes the header to w with the specified status code.
func (handler *UnroutedHandler) sendResp(w http.ResponseWriter, r *http.Request, status int) {
	w.WriteHeader(status)

	handler.log("ResponseOutgoing", "status", strconv.Itoa(status), "method", r.Method, "path", r.URL.Path)
}

// Make an absolute URLs to the given upload id. If the base path is absolute
// it will be prepended else the host and protocol from the request is used.
func (handler *UnroutedHandler) absFileURL(r *http.Request, id string) string {
	if handler.isBasePathAbs {
		return handler.basePath + id
	}

	// Read origin and protocol from request
	host, proto := getHostAndProtocol(r, handler.config.RespectForwardedHeaders)

	url := proto + "://" + host + handler.basePath + id

	return url
}

type progressWriter struct {
	Offset int64
}

func (w *progressWriter) Write(b []byte) (int, error) {
	atomic.AddInt64(&w.Offset, int64(len(b)))
	return len(b), nil
}

// sendProgressMessage will send a notification over the UploadProgress channel
// every second, indicating how much data has been transfered to the server.
// It will stop sending these instances once the returned channel has been
// closed. The returned reader should be used to read the request body.
func (handler *UnroutedHandler) sendProgressMessages(info FileInfo, reader io.Reader) (io.Reader, chan<- struct{}) {
	progress := &progressWriter{
		Offset: info.Offset,
	}
	stop := make(chan struct{}, 1)
	reader = io.TeeReader(reader, progress)

	go func() {
		for {
			select {
			case <-stop:
				info.Offset = atomic.LoadInt64(&progress.Offset)
				handler.UploadProgress <- info
				return
			case <-time.After(1 * time.Second):
				info.Offset = atomic.LoadInt64(&progress.Offset)
				handler.UploadProgress <- info
			}
		}
	}()

	return reader, stop
}

// getHostAndProtocol extracts the host and used protocol (either HTTP or HTTPS)
// from the given request. If `allowForwarded` is set, the X-Forwarded-Host,
// X-Forwarded-Proto and Forwarded headers will also be checked to
// support proxies.
func getHostAndProtocol(r *http.Request, allowForwarded bool) (host, proto string) {
	if r.TLS != nil {
		proto = "https"
	} else {
		proto = "http"
	}

	host = r.Host

	if !allowForwarded {
		return
	}

	if h := r.Header.Get("X-Forwarded-Host"); h != "" {
		host = h
	}

	if h := r.Header.Get("X-Forwarded-Proto"); h == "http" || h == "https" {
		proto = h
	}

	if h := r.Header.Get("Forwarded"); h != "" {
		if r := reForwardedHost.FindStringSubmatch(h); len(r) == 2 {
			host = r[1]
		}

		if r := reForwardedProto.FindStringSubmatch(h); len(r) == 2 {
			proto = r[1]
		}
	}

	return
}

// The get sum of all sizes for a list of upload ids while checking whether
// all of these uploads are finished yet. This is used to calculate the size
// of a final resource.
func (handler *UnroutedHandler) sizeOfUploads(ids []string) (size int64, err error) {
	for _, id := range ids {
		info, err := handler.composer.Core.GetInfo(id)
		if err != nil {
			return size, err
		}

		if info.SizeIsDeferred || info.Offset != info.Size {
			err = ErrUploadNotFinished
			return size, err
		}

		size += info.Size
	}

	return
}

// Verify that the Upload-Length and Upload-Defer-Length headers are acceptable for creating a
// new upload
func (handler *UnroutedHandler) validateNewUploadLengthHeaders(uploadLengthHeader string, uploadDeferLengthHeader string) (uploadLength int64, uploadLengthDeferred bool, err error) {
	haveBothLengthHeaders := uploadLengthHeader != "" && uploadDeferLengthHeader != ""
	haveInvalidDeferHeader := uploadDeferLengthHeader != "" && uploadDeferLengthHeader != UploadLengthDeferred
	lengthIsDeferred := uploadDeferLengthHeader == UploadLengthDeferred

	if lengthIsDeferred && !handler.composer.UsesLengthDeferrer {
		err = ErrNotImplemented
	} else if haveBothLengthHeaders {
		err = ErrUploadLengthAndUploadDeferLength
	} else if haveInvalidDeferHeader {
		err = ErrInvalidUploadDeferLength
	} else if lengthIsDeferred {
		uploadLengthDeferred = true
	} else {
		uploadLength, err = strconv.ParseInt(uploadLengthHeader, 10, 64)
		if err != nil || uploadLength < 0 {
			err = ErrInvalidUploadLength
		}
	}

	return
}

// ParseMetadataHeader parses the Upload-Metadata header as defined in the
// File Creation extension.
// e.g. Upload-Metadata: name bHVucmpzLnBuZw==,type aW1hZ2UvcG5n
func ParseMetadataHeader(header string) map[string]string {
	meta := make(map[string]string)

	for _, element := range strings.Split(header, ",") {
		element := strings.TrimSpace(element)

		parts := strings.Split(element, " ")

		// Do not continue with this element if no key and value or presented
		if len(parts) != 2 {
			continue
		}

		// Ignore corrent element if the value is no valid base64
		key := parts[0]
		value, err := base64.StdEncoding.DecodeString(parts[1])
		if err != nil {
			continue
		}

		meta[key] = string(value)
	}

	return meta
}

// SerializeMetadataHeader serializes a map of strings into the Upload-Metadata
// header format used in the response for HEAD requests.
// e.g. Upload-Metadata: name bHVucmpzLnBuZw==,type aW1hZ2UvcG5n
func SerializeMetadataHeader(meta map[string]string) string {
	header := ""
	for key, value := range meta {
		valueBase64 := base64.StdEncoding.EncodeToString([]byte(value))
		header += key + " " + valueBase64 + ","
	}

	// Remove trailing comma
	if len(header) > 0 {
		header = header[:len(header)-1]
	}

	return header
}

// Parse the Upload-Concat header, e.g.
// Upload-Concat: partial
// Upload-Concat: final;http://tus.io/files/a /files/b/
func parseConcat(header string) (isPartial bool, isFinal bool, partialUploads []string, err error) {
	if len(header) == 0 {
		return
	}

	if header == "partial" {
		isPartial = true
		return
	}

	l := len("final;")
	if strings.HasPrefix(header, "final;") && len(header) > l {
		isFinal = true

		list := strings.Split(header[l:], " ")
		for _, value := range list {
			value := strings.TrimSpace(value)
			if value == "" {
				continue
			}

			id, extractErr := extractIDFromPath(value)
			if extractErr != nil {
				err = extractErr
				return
			}

			partialUploads = append(partialUploads, id)
		}
	}

	// If no valid partial upload ids are extracted this is not a final upload.
	if len(partialUploads) == 0 {
		isFinal = false
		err = ErrInvalidConcat
	}

	return
}

// extractIDFromPath pulls the last segment from the url provided
func extractIDFromPath(url string) (string, error) {
	result := reExtractFileID.FindStringSubmatch(url)
	if len(result) != 2 {
		return "", ErrNotFound
	}
	return result[1], nil
}

func i64toa(num int64) string {
	return strconv.FormatInt(num, 10)
}
