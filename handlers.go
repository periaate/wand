package wand

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"
)

const ErrCode = http.StatusUnauthorized

func LinkHandler(linkBuilder LinkBuilder) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ld, err := parseParams(r.URL.Query())
		if err != nil {
			slog.Error("error parsing parameters", "err", err)
			RenderLinkIndex(w, "", http.StatusBadRequest, err)
		}
		res, err := linkBuilder(ld)
		if err != nil {
			slog.Error("error building link", "err", err)
			RenderLinkIndex(w, "", http.StatusBadRequest, err)
		}
		RenderLinkIndex(w, res, http.StatusOK, nil)
	})
}

func SessionProxy(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Path[1:]

	sessionCookie, err := r.Cookie("session")
	if err != nil && len(token) != tokSLen {
		http.Error(w, "", http.StatusUnauthorized)
		return
	}
	if len(token) == tokSLen {
		sessionCookie, err = tryAuthLink(token)
		if err != nil {
			slog.Error("Session token was invalid", "error", err)
			http.Error(w, "", http.StatusUnauthorized)
			return
		}

		http.SetCookie(w, sessionCookie)
	}

	session, err := getSession(sessionCookie.Value)
	if err != nil {
		slog.Error("No such session", "error", err)
		http.Error(w, "", http.StatusUnauthorized)
		return
	}

	if !session.IsValid() {
		slog.Error("Session is not valid", "error", err)
		http.Error(w, "", http.StatusUnauthorized)
		return
	}

	http.Redirect(w, r, getRedirectURL(), http.StatusFound)
}

func getRedirectURL() string {
	dom := os.Getenv("WAND_DOMAIN")
	return fmt.Sprintf("https://%s/", dom)
}

func tryAuthLink(link string) (cookie *http.Cookie, err error) {
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
