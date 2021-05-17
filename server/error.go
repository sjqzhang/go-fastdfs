package server

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
