// Package tus provides a client to tus protocol version 1.0.0.
//
// tus is a protocol based on HTTP for resumable file uploads. Resumable means that
// an upload can be interrupted at any moment and can be resumed without
// re-uploading the previous data again. An interruption may happen willingly, if
// the user wants to pause, or by accident in case of an network issue or server
// outage (http://tus.io).
package tus
