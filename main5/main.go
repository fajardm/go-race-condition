package main

import (
	"errors"
	"fmt"
	"log"

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
	pool := goredis.NewPool(client)
	rs := redsync.New(pool)
	mutex := rs.NewMutex("locker")

	app.Get("/vouchers/:id", func(c *fiber.Ctx) error {
		// ctx, cancel := context.WithTimeout(c.Context(), 7*time.Second)
		// defer cancel()

		id := c.Params("id")

		if err := mutex.Lock(); err != nil {
			fmt.Println("error1", err)
			return err
		}

		defer func() {
			if _, err := mutex.Unlock(); err != nil {
				fmt.Println("error2", err)
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
			return err
		}

		return c.JSON(quotaUpdated)
	})

	log.Fatal(app.Listen(":3001"))
}
