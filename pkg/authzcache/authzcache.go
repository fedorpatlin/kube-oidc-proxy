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
	value *[]byte
}

type OPACache struct {
	mux       *sync.Mutex
	cache     map[string]*list.Element
	evictList *list.List
	hashImpl  hash.Hash
	count     int
}

func NewOPACache() *OPACache {
	c := OPACache{
		mux:       &sync.Mutex{},
		hashImpl:  crypto.SHA256.New(),
		cache:     map[string]*list.Element{},
		evictList: list.New(),
		count:     0,
	}
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

func (c *OPACache) Put(key string, val *[]byte) error {
	key, err := c.getStringHash(key)
	if err != nil {
		return err
	}
	c.mux.Lock()
	defer c.mux.Unlock()
	if c.count > maxCachedObjects {
		c.evictLeastUsed()
	}
	newEntry := listEntry{key: key, value: val}
	newElement := c.evictList.PushFront(newEntry)
	c.count += 1
	c.cache[key] = newElement
	return nil
}

func (c *OPACache) Get(key string) (*[]byte, bool) {
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
	for k := range c.cache {
		delete(c.cache, k)
	}
	c.evictList.Init()
}

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
