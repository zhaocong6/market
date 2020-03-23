package market

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"io/ioutil"
)

func decode(msg []byte) ([]byte, error) {
	reader := flate.NewReader(bytes.NewReader(msg))
	defer reader.Close()
	return ioutil.ReadAll(reader)
}

func gzipDecode(msg []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(msg))

	if err != nil {
		return nil, err
	}
	defer reader.Close()

	return ioutil.ReadAll(reader)
}
