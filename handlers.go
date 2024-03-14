package wand

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httputil"
	"time"
)

func LinkHandler(domain string, invalid ...string) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		link, err := parseParams(r.URL.Query())
		if err != nil {
			RenderLinkIndex(w, "", http.StatusBadRequest, err)
			return
		}

		for _, inv := range invalid {
			if inv == link.target.String() {
				RenderLinkIndex(w, "", http.StatusBadRequest, fmt.Errorf("invalid target URL"))
				return
			}
		}

		key := keygen()
		mutex.Lock()
		defer mutex.Unlock()
		linkMap[key] = link

		// fmt.Fprintf(w, "https://%s/?action=auth&link=%s", domain, key)
		RenderLinkIndex(w, fmt.Sprintf("https://%s/%s", domain, key), http.StatusOK, nil)
	})
}

func SessionMW(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Path[1:]

	sessionCookie, err := r.Cookie("session")
	if err != nil && len(token) != tokSLen {
		http.Error(w, "", http.StatusUnauthorized)
		return
	}
	if len(token) == tokSLen {
		sessionCookie, err = auth(token)
		if err != nil {
			fmt.Println("Session token was invalid", err)
			http.Error(w, "", http.StatusUnauthorized)
			return
		}

		http.SetCookie(w, sessionCookie)
	}

	session, err := getSession(sessionCookie.Value)
	if err != nil {
		fmt.Println("No such session", err)
		http.Error(w, "", http.StatusUnauthorized)
		return
	}

	if !session.IsValid() {
		fmt.Println("Session is not valid")
		http.Error(w, "", http.StatusUnauthorized)
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(session.targetURL)
	proxy.ServeHTTP(w, r)
}

func auth(link string) (cookie *http.Cookie, err error) {
	linkData, exists := linkMap[link]
	if !exists {
		err = fmt.Errorf("no such link")
		return
	}

	if linkData.lasts.Before(time.Now()) {
		err = fmt.Errorf("link has expired")
		return
	}

	if linkData.usable == 0 {
		err = fmt.Errorf("link has been used up")
		return
	}

	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return
	}

	sessionID := base64.URLEncoding.EncodeToString(b)

	mutex.Lock()
	defer mutex.Unlock()
	sessionMap[sessionID] = sessionData{
		validUntil: time.Now().Add(linkData.duration),
		targetURL:  linkData.target,
	}

	linkData.mu.Lock()
	defer linkData.mu.Unlock()
	linkData.sessions = append(linkData.sessions, sessionID)
	linkData.usable--

	// cookie
	cookie = &http.Cookie{
		Name:    "session",
		Value:   sessionID,
		Expires: time.Now().Add(linkData.duration),
	}

	return cookie, nil
}
