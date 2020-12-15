package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/cornelk/hashmap"
	"github.com/go-redis/redis/v8"
	"github.com/go-redsync/redsync/v4"
	"github.com/go-redsync/redsync/v4/redis/goredis/v8"
	"github.com/gofiber/fiber/v2"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

type Voucher struct {
	ID    string `db:"id"`
	Quota int    `db:"quota"`
}

type Repository struct {
	db *sqlx.DB
}

func (r *Repository) Get(id string) (*Voucher, error) {
	var voucher Voucher
	err := r.db.QueryRowx(`SELECT id, quota FROM vouchers WHERE id=$1`, id).StructScan(&voucher)
	return &voucher, err
}

func (r *Repository) Update(id string, quota int) error {
	_, err := r.db.Exec("UPDATE vouchers SET quota=$1 WHERE id=$2", quota, id)
	return err
}

func main() {
	db, err := sqlx.Connect("postgres", "user=root password=secret dbname=gobackend sslmode=disable")
	if err != nil {
		log.Fatalln(err)
	}

	repo := Repository{db: db}

	app := fiber.New()

	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	locker := NewRedsync(client)

	app.Get("/vouchers/:id", func(c *fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(c.Context(), 10*time.Second)
		defer cancel()

		id := c.Params("id")

		if err := locker.Lock(ctx, id); err != nil {
			fmt.Println("error1", err)
			return err
		}
		defer func() {
			if err := locker.Unlock(ctx, id); err != nil {
				fmt.Println("error2", err)
			}
		}()

		existing, err := repo.Get(id)
		if err != nil {
			return err
		}
		fmt.Println("existing", existing.Quota)

		quotaUpdated := existing.Quota - 1

		if err := repo.Update(id, quotaUpdated); err != nil {
			return err
		}

		return c.JSON(quotaUpdated)
	})

	log.Fatal(app.Listen(":3001"))
}

type redsyncLock struct {
	driver *redsync.Redsync
	hash   *hashmap.HashMap
}

func NewRedsync(client *redis.Client) *redsyncLock {
	pool := goredis.NewPool(client)
	driver := redsync.New(pool)
	hash := new(hashmap.HashMap)
	return &redsyncLock{driver, hash}
}

func (l redsyncLock) Lock(ctx context.Context, key string) error {
	locker := l.driver.NewMutex(key)
	if err := locker.LockContext(ctx); err != nil {
		return err
	}
	l.hash.Set(key, locker)
	return nil
}

func (l redsyncLock) Unlock(ctx context.Context, key string) error {
	_locker, ok := l.hash.Get(key)
	if !ok {
		return nil
	}

	locker, ok := _locker.(*redsync.Mutex)
	if !ok {
		return nil
	}

	l.hash.Del(key)

	if _, err := locker.Unlock(); err != nil {
		return err
	}
	return nil
}
