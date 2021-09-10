package webservice

import "net/http"

func setAllowAllCorsHeaders(w *http.ResponseWriter, req *http.Request) {
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
	(*w).Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
	(*w).Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
}

// AllowAllCorsMiddleware returns middleware function that can be used in router.Use()
func AllowAllCorsMiddleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setAllowAllCorsHeaders(&w, r)
		h.ServeHTTP(w, r)
	})
}
