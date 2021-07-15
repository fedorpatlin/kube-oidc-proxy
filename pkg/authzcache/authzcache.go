// Copyright Jetstack Ltd. See LICENSE for details.

package authzcache

import (
	"container/list"
	"crypto"

	_ "crypto/sha256"
	"encoding/base64"
	"fmt"
	"hash"
	"sync"
)

const maxCachedObjects int = 1024

type listEntry struct {
	key   string
	value []byte
}

type OPACache struct {
	mux       *sync.Mutex
	cache     map[string]*list.Element
	evictList *list.List
	hashImpl  hash.Hash
	count     int
}

func NewOPACache() *OPACache {
	c := OPACache{}
	c.mux = &sync.Mutex{}
	c.hashImpl = crypto.SHA256.New()
	c.cache = map[string]*list.Element{}
	c.evictList = list.New()
	c.count = 0

	return &c
}

func (c *OPACache) getStringHash(toHash string) (string, error) {
	_, err := c.hashImpl.Write([]byte(toHash))
	if err != nil {
		return "", err
	}
	defer c.hashImpl.Reset()
	return base64.URLEncoding.EncodeToString(c.hashImpl.Sum(nil)), nil
}

func (c *OPACache) Put(keystr string, val []byte) error {
	if _, ok := c.Get(keystr); ok {
		return nil
	}
	key, err := c.getStringHash(keystr)
	if err != nil {
		return err
	}
	c.mux.Lock()
	defer c.mux.Unlock()
	c.put(key, val)
	return nil
}
func (c *OPACache) put(key string, val []byte) {
	if c.count >= maxCachedObjects {
		c.evictLeastUsed()
	}
	newEntry := listEntry{key: key, value: val}
	newElement := c.evictList.PushFront(newEntry)
	c.count += 1
	c.cache[key] = newElement
}

func (c *OPACache) Get(key string) ([]byte, bool) {
	key, err := c.getStringHash(key)
	if err != nil {
		return nil, false
	}
	c.mux.Lock()
	defer c.mux.Unlock()
	found, ok := c.cache[key]
	if !ok {
		return nil, ok
	}
	c.evictList.MoveToFront(found)
	result := found.Value.(listEntry)
	return result.value, ok
}

func (c *OPACache) Prune() {
	c.mux.Lock()
	defer c.mux.Unlock()
	for k := range c.cache {
		delete(c.cache, k)
	}
	c.evictList.Init()
	c.count = 0
}

// вызывается только из синхронизированного кода, поэтому лочить явно тут ничего не надо
func (c *OPACache) evictLeastUsed() {
	evictElement := c.evictList.Back()
	if evictElement == nil {
		panic(fmt.Errorf("list element must not be nil"))
	}
	c.evictList.Remove(evictElement)
	deleteme := evictElement.Value.(listEntry)
	delete(c.cache, deleteme.key)
	c.count = c.count - 1
}
