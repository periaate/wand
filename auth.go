package wand

import (
	"fmt"
	"net/url"
	"strconv"
	"sync"
	"time"
)

const (
	defDuration = 30 * time.Minute // Default duration for which sessions generated by a link are valid.
	maxDuration = 180 * time.Minute
	minDuration = 1 * time.Minute

	defExpiry  = 5 * time.Minute // Default expiry time for a link.
	maxExpiry  = 10 * time.Minute
	minExpirty = 30 * time.Second

	defUses = 1 // Default number of times a link can be used.
	maxUses = 5
	minUses = 1

	tokBLen = 32
)

var tokSLen = getB64Len(tokBLen)

// sessionMap stores the session information.
var sessionMap = make(map[string]sessionData)
var mutex = &sync.Mutex{}
var linkMap = make(map[string]*LinkData)

// sessionData holds the session information along with the expiry time.
type sessionData struct {
	validUntil time.Time
	targetURL  *url.URL
}

// IsValid checks if the session is valid.
func (s sessionData) IsValid() bool { return time.Now().Before(s.validUntil) }

type LinkData struct {
	usable   int           // The number of times this link can be used.
	lasts    time.Time     // The time this link lasts.
	duration time.Duration // The duration for which sessions generated by this link are valid.
	sessions []string      // The sessions generated by this link.
	target   *url.URL      // The target URL.
	mu       sync.Mutex
}

func RunSessionWorker() {
	for {
		time.Sleep(1 * time.Minute)
		go func() {
			mutex.Lock()
			defer mutex.Unlock()
			for key, session := range sessionMap {
				if time.Now().After(session.validUntil) {
					delete(sessionMap, key)
				}
			}
		}()
	}
}

func getSession(sessionID string) (sessionData, error) {
	mutex.Lock()
	defer mutex.Unlock()

	session, exists := sessionMap[sessionID]
	if !exists {
		return sessionData{}, fmt.Errorf("no such session")
	}

	return session, nil
}

func parseParams(r url.Values) (link *LinkData, err error) {
	target := r.Get("target")
	if len(target) == 0 {
		err = fmt.Errorf("target URL not provided")
		return
	}
	prefix := "http://"
	if r.Has("TLS") {
		prefix = "https://"
	}
	target = prefix + target
	URL, err := url.Parse(target)
	if err != nil {
		err = fmt.Errorf("error parsing target URL: %w", err)
		return
	}

	linkExpiresIn := Or(time.ParseDuration(r.Get("expires_in")))(defExpiry)
	linkExpiresIn = Clamp(linkExpiresIn, minDuration, maxDuration)

	uses := Or(strconv.Atoi(r.Get("link_uses")))(defUses)
	uses = Clamp(uses, minUses, maxUses)

	sessionLasts := Or(time.ParseDuration(r.Get("ses_duration")))(defDuration)
	sessionLasts = Clamp(sessionLasts, minDuration, maxDuration)

	link = &LinkData{
		target:   URL,
		usable:   uses,
		sessions: []string{},
		lasts:    time.Now().Add(linkExpiresIn),
		duration: time.Duration(sessionLasts),
	}

	return link, nil
}
