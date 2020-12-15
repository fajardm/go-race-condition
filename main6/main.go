package main

import (
	"context"
	"errors"
	"fmt"
	locker2 "github.com/go-race-condition/pkg/locker"
	"log"
	"time"

	"github.com/go-redis/redis/v8"
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

type UpdateQuotaFunc func(ctx context.Context, id string) (int, error)

func NewUpdateQuota(repo Repository) UpdateQuotaFunc {
	return func(ctx context.Context, id string) (int, error) {
		existing, err := repo.Get(id)
		if err != nil {
			return 0, err
		}
		if existing.Quota <= 0 {
			err := errors.New("no quota")
			fmt.Println(err)
			return 0, err
		}
		fmt.Println("existing", existing.Quota)

		quotaUpdated := existing.Quota - 1

		if err := repo.Update(id, quotaUpdated); err != nil {
			return 0, err
		}
		return quotaUpdated, nil
	}
}

func NewUpdateQuotaLocker(next UpdateQuotaFunc, locker locker2.Locker) UpdateQuotaFunc {
	return func(ctx context.Context, id string) (int, error) {
		err := locker.Lock(ctx, id, time.Second*5)
		if err != nil {
			//log.Println("err locked", err)
			return 0, err
		}

		defer func() {
			err = locker.Unlock(ctx, id)
			if err != nil {
				log.Println("err unlock", err)
			}
		}()

		return next(ctx, id)
	}
}

func NewRetryUpdateQuota(next UpdateQuotaFunc) UpdateQuotaFunc {
	return func(ctx context.Context, id string) (res int, err error) {
		for i := 0; i < 20; i++ {
			res, err = next(ctx, id)
			if err == nil {
				return
			}
			time.Sleep(time.Millisecond * 50)
		}
		return
	}
}

func main() {
	db, err := sqlx.Connect("postgres", "user=postgres password=secret dbname=anu sslmode=disable")
	if err != nil {
		log.Fatalln(err)
	}

	repo := Repository{db: db}

	app := fiber.New()

	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	locker := locker2.NewRedisLocker(client)

	updateQuota := NewUpdateQuota(repo)
	updateQuota = NewUpdateQuotaLocker(updateQuota, locker)
	updateQuota = NewRetryUpdateQuota(updateQuota)

	app.Get("/vouchers/:id", func(c *fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(c.Context(), 7*time.Second)
		defer cancel()

		id := c.Params("id")

		quotaUpdated, err := updateQuota(ctx, id)
		if err != nil {
			return err
		}

		return c.JSON(quotaUpdated)
	})

	log.Fatal(app.Listen(":3000"))
}
