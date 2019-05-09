package tus

type Store interface {
	Get(fingerprint string) (string, bool)
	Set(fingerprint, url string)
	Delete(fingerprint string)
	Close()
}
