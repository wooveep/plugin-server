package main

import "bytes"

func unifySSEChunk(data []byte) []byte {
	if bytes.IndexByte(data, '\r') < 0 {
		return data
	}
	data = bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
	data = bytes.ReplaceAll(data, []byte("\r"), []byte("\n"))
	return data
}
