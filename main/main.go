package main

import (
	"time"

	"github.com/lusoalex/oauth2-provider"
)

func main() {
	oauth2Provider.Serve(&oauth2Provider.Oauth2ServerOptions{
		Kvs:  NewFakeKeyValueStore(),
		Port: "8000",
	})
}

type FakeKeyValueStore struct {
	code           map[string][]byte
	codeExpiration map[string]time.Time
}

func (t *FakeKeyValueStore) Set(key, value []byte, d time.Duration) error {
	expiration := time.Now().Add(d)
	sKey := string(key)
	t.code[sKey] = value
	t.codeExpiration[sKey] = expiration
	return nil
}

func (t *FakeKeyValueStore) Get(key []byte) ([]byte, error) {
	sKey := string(key)
	expired := t.codeExpiration[string(key)]

	if time.Now().Before(expired) {
		return t.code[sKey], nil
	}

	return nil, nil
}

func (t *FakeKeyValueStore) Del(key []byte) ([]byte, error) {
	sKey := string(key)
	res := t.code[sKey]
	delete(t.code, sKey)
	return res, nil
}

func NewFakeKeyValueStore() *FakeKeyValueStore {
	return &FakeKeyValueStore{
		code:           make(map[string][]byte),
		codeExpiration: make(map[string]time.Time),
	}
}
