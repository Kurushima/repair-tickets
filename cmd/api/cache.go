package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

const ticketCacheTTL = time.Minute

var errCacheUnavailable = errors.New("ticket cache unavailable")

func ticketCacheKey(id int32) string {
	return fmt.Sprintf("ticket:%d", id)
}

func (s *ticketStore) lookup(ctx context.Context, id int32) (ticket, error) {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()

	key := ticketCacheKey(id)
	var found ticket
	data, err := s.cache.Get(ctx, key).Bytes()
	if err == nil {
		if err := json.Unmarshal(data, &found); err == nil {
			log.Printf("cache HIT %s", key)
			return found, nil
		}
		log.Printf("cache invalid JSON %s", key)
	} else if !errors.Is(err, redis.Nil) {
		log.Printf("cache GET %s: %v", key, err)
	}
	log.Printf("cache MISS %s", key)

	err = s.db.QueryRow(ctx, `
		SELECT id, description, status FROM tickets WHERE id = $1
	`, id).Scan(&found.ID, &found.Description, &found.Status)
	if err != nil {
		return ticket{}, err
	}
	data, err = json.Marshal(found)
	if err != nil {
		log.Printf("Encode cache %s: %v", key, err)
		return found, nil
	}
	if err := s.cache.Set(ctx, key, data, ticketCacheTTL).Err(); err != nil {
		log.Printf("cache SET %s: %v", key, err)
	}
	return found, nil
}

func (s *ticketStore) invalidateCache(ctx context.Context, id int32) error {
	key := ticketCacheKey(id)
	if err := s.cache.Del(ctx, key).Err(); err != nil {
		log.Printf("cache DEL %s: %v", key, err)
		return errCacheUnavailable
	}
	log.Printf("cache INVALIDATE %s", key)
	return nil
}
