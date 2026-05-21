package httpx

import "net/http"

type ErrorResponse struct {
	Error string `json:"error"`
	Code  string `json:"code,omitempty"`
}

func WriteError(w http.ResponseWriter, status int, message, code string) {
	WriteJSON(w, status, ErrorResponse{Error: message, Code: code})
}

func WriteBadRequest(w http.ResponseWriter, msg string) {
	WriteError(w, http.StatusBadRequest, msg, "bad_request")
}

func WriteUnauthorized(w http.ResponseWriter) {
	WriteError(w, http.StatusUnauthorized, "Unauthorized", "unauthorized")
}

func WriteInternal(w http.ResponseWriter) {
	WriteError(w, http.StatusInternalServerError, "Internal server error", "internal")
}
