package session

import (
	"net/http"
)

//Handler 增加一个session参数
type Handler interface {
	ServeHTTP(*Session, http.ResponseWriter, *http.Request)
}

//HandlerFunc x
type HandlerFunc func(s *Session, w http.ResponseWriter, r *http.Request)

// ServeHTTP calls f(w, r).
func (f HandlerFunc) ServeHTTP(s *Session, w http.ResponseWriter, r *http.Request) {
	f(s, w, r)
}

//Auth 令牌检查
func Auth(inner Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess := SessionMgr().GetSession(w, r)
		if sess == nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		inner.ServeHTTP(sess, w, r)
	})
}
