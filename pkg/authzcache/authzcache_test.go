// Copyright Jetstack Ltd. See LICENSE for details.

package authzcache

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"strconv"
	"testing"
	"time"
)

type TestValue struct {
	Name  string
	Value string
}

func (v TestValue) CalculateKey() string {
	return v.Name
}

func TestPutValue(t *testing.T) {
	valueToCache := TestValue{
		Name:  "Test",
		Value: "Value",
	}
	cache := NewOPACache()
	cached, _ := json.Marshal(valueToCache)
	if err := cache.Put(valueToCache.CalculateKey(), cached); err != nil {
		t.Error(err.Error())
	}
	valueRestore := TestValue{
		Name: valueToCache.Name,
	}
	nv, ok := cache.Get(valueRestore.CalculateKey())
	if !ok {
		t.Fatalf("cached value must exists but no")
	}
	err := json.Unmarshal(nv, &valueRestore)
	if err != nil {
		t.Error(err.Error())
	}
	if valueRestore.Value != "Value" {
		t.Error("Wrong value returned from cache")
	}
}

func prepareValues(b *testing.B) []TestValue {
	b.Helper()
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
	cachedValues := make([]TestValue, b.N)
	for n := 0; n < b.N; n++ {
		cachedValues[n] = TestValue{
			Name:  fmt.Sprintf("%d", rnd.Int31()),
			Value: fmt.Sprintf("%d", rnd.Int31()),
		}
	}
	return cachedValues
}

func BenchmarkPut(b *testing.B) {
	testValues := prepareValues(b)
	cache := func() *OPACache {
		b.Helper()
		return NewOPACache()
	}()
	for _, v := range testValues {
		cached, _ := json.Marshal(v)
		cache.Put(v.CalculateKey(), cached)
		val, ok := cache.Get(v.CalculateKey())
		if !ok {
			b.Fail()
		}
		restoredVal := TestValue{}
		json.Unmarshal(val, &restoredVal)
		if restoredVal.Value != v.Value {
			b.Fail()
		}
	}
	b.Logf("cache length %d", len(cache.cache))
}

func TestPrune(t *testing.T) {
	cache := NewOPACache()
	key := "some key"
	val := []byte("some value")
	cache.Put(key, val)
	cache.Prune()
	_, ok := cache.Get(key)
	if cache.evictList.Len() > 0 {
		t.Fail()
	}
	if ok {
		t.Fail()
	}
	if len(cache.cache) > 0 {
		t.Fail()
	}
	if cache.count > 0 {
		t.Fail()
	}
}

func TestEviction(t *testing.T) {
	cache := NewOPACache()
	rndg := rand.New(rand.NewSource(int64(time.Now().Nanosecond())))
	key := func() string { return strconv.FormatInt(rndg.Int63(), 16) }
	val := []byte("testval")
	for i := 0; i < maxCachedObjects+100; i++ {
		cache.Put(key(), val)
	}
	if len(cache.cache) > maxCachedObjects {
		t.Fatalf("cached objects count > maxCachedObjects: %d > %d", len(cache.cache), maxCachedObjects)
	}
	if cache.count > maxCachedObjects {
		t.Fail()
	}
	if cache.evictList.Len() > maxCachedObjects {
		t.Fail()
	}
}
