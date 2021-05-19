/*
Package handler provides ways to accept tus 1.0 calls using HTTP.

tus is a protocol based on HTTP for resumable file uploads. Resumable means that
an upload can be interrupted at any moment and can be resumed without
re-uploading the previous data again. An interruption may happen willingly, if
the user wants to pause, or by accident in case of an network issue or server
outage (http://tus.io).

The basics of tusd

tusd was designed in way which allows an flexible and customizable usage. We
wanted to avoid binding this package to a specific storage system – particularly
a proprietary third-party software. Therefore tusd is an abstract layer whose
only job is to accept incoming HTTP requests, validate them according to the
specification and finally passes them to the data store.

The data store is another important component in tusd's architecture whose
purpose is to do the actual file handling. It has to write the incoming upload
to a persistent storage system and retrieve information about an upload's
current state. Therefore it is the only part of the system which communicates
directly with the underlying storage system, whether it be the local disk, a
remote FTP server or cloud providers such as AWS S3.

Using a store composer

The only hard requirements for a data store can be found in the DataStore
interface. It contains methods for creating uploads (NewUpload), writing to
them (WriteChunk) and retrieving their status (GetInfo). However, there
are many more features which are not mandatory but may still be used.
These are contained in their own interfaces which all share the *DataStore
suffix. For example, GetReaderDataStore which enables downloading uploads or
TerminaterDataStore which allows uploads to be terminated.

The store composer offers a way to combine the basic data store - the core -
implementation and these additional extensions:

  composer := tusd.NewStoreComposer()
  composer.UseCore(dataStore) // Implements DataStore
  composer.UseTerminater(terminater) // Implements TerminaterDataStore
  composer.UseLocker(locker) // Implements LockerDataStore

The corresponding methods for adding an extension to the composer are prefixed
with Use* followed by the name of the corresponding interface. However, most
data store provide multiple extensions and adding all of them manually can be
tedious and error-prone. Therefore, all data store distributed with tusd provide
an UseIn() method which does this job automatically. For example, this is the
S3 store in action (see S3Store.UseIn):

  store := s3store.New(…)
  locker := memorylocker.New()
  composer := tusd.NewStoreComposer()
  store.UseIn(composer)
  locker.UseIn(composer)

Finally, once you are done with composing your data store, you can pass it
inside the Config struct in order to create create a new tusd HTTP handler:

  config := tusd.Config{
    StoreComposer: composer,
    BasePath: "/files/",
  }
  handler, err := tusd.NewHandler(config)

This handler can then be mounted to a specific path, e.g. /files:

  http.Handle("/files/", http.StripPrefix("/files/", handler))
*/
package handler
