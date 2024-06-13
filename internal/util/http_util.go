package util

import "net/http"

func WriteResponse(w http.ResponseWriter, statusCode int, message string) {
	w.WriteHeader(statusCode)
	w.Write([]byte(message))
}

func GetResponseHeader(w http.ResponseWriter) {
	w.Header().Add("status-code", "200")
	w.Header().Add("Content-Type", "application/json")
	w.Header().Add("charset", "utf-8")
}
