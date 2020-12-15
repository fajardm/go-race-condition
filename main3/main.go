package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/cornelk/hashmap"
	"github.com/go-zookeeper/zk"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/timeout"
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

	zkConn, chanEvent, err := zk.Connect([]string{"127.0.0.1:2181"}, 2*time.Second)
	if err != nil {
		log.Fatalln(err)
	}

	go func() {
		for {
			e := <-chanEvent
			fmt.Println(e)
		}
	}()

	// zkLock := zk.NewLock(zkConn, "/locker", zk.WorldACL(zk.PermAll))
	// if err != nil {
	// 	log.Fatalln(err)
	// }

	locker := &Locker{conn: zkConn, hash: new(hashmap.HashMap)}

	repo := Repository{db: db}

	app := fiber.New()

	app.Get("/vouchers/:id", timeout.New(func(c *fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(c.Context(), 10*time.Second)
		defer cancel()

		id := c.Params("id")

		if err := locker.Lock(ctx, id); err != nil {
			fmt.Println("lock err", err)
			return err
		}
		defer func() {
			if err := locker.Unlock(ctx, id); err != nil {
				fmt.Println("unlock err", err)
			}
		}()

		existing, err := repo.Get(id)
		if err != nil {
			return err
		}
		if existing.Quota <= 0 {
			err := errors.New("no quota")
			fmt.Println(err)
			return err
		}
		fmt.Println("existing", existing.Quota)

		quotaUpdated := existing.Quota - 1

		if err := repo.Update(id, quotaUpdated); err != nil {
			fmt.Println("err update", err)
			return err
		}

		return c.JSON(quotaUpdated)
	}, 5*time.Second))

	log.Fatal(app.Listen(":3001"))
}

type Locker struct {
	conn *zk.Conn
	hash *hashmap.HashMap
}

func (l Locker) Lock(ctx context.Context, key string) error {
	type channelr struct {
		lock *zk.Lock
		err  error
	}

	c := make(chan *channelr)

	go func() {
		locker := zk.NewLock(l.conn, fmt.Sprintf("/%s", key), zk.WorldACL(zk.PermAll))
		c <- &channelr{err: locker.Lock(), lock: locker}
	}()

	select {
	case <-ctx.Done():
		return errors.New("timeout")
	case c := <-c:
		if c.err != nil {
			return c.err
		}
		l.hash.Set(key, c.lock)
		return nil
	}
}

func (l Locker) Unlock(ctx context.Context, key string) error {
	_locker, ok := l.hash.Get(key)
	if !ok {
		return nil
	}
	locker, ok := _locker.(*zk.Lock)
	if !ok {
		return nil
	}
	l.hash.Del(key)
	return locker.Unlock()
}
