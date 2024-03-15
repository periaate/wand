package wand

import (
	"crypto/rand"
	"encoding/base64"

	"golang.org/x/exp/constraints"
)

// Or returns the first result if the second is not the zero value, otherwise it returns the curried argument.
func Or[T any, V comparable](res T, v V) func(def T) T {
	return func(def T) T {
		if v != *new(V) {
			return def
		}
		return res
	}
}

func Clamp[T constraints.Ordered](val, lower, upper T) (res T) {
	if val > upper {
		return upper
	}
	if val < lower {
		return lower
	}
	return val
}

func keygen() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.URLEncoding.EncodeToString(b)
}
