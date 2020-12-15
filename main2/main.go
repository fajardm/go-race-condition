package main

import (
	"context"
	"fmt"
	"log"

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

type Request struct {
	id     string
	result chan int
}

func QuotaWorker(ctx context.Context, repo Repository, request chan *Request) {
	for {
		select {
		case request := <-request:
			existing, _ := repo.Get(request.id)
			quota := existing.Quota - 1
			_ = repo.Update(request.id, quota)
			request.result <- quota
		case <-ctx.Done():
			fmt.Println("timeout")
		}
	}
}

func main() {
	db, err := sqlx.Connect("postgres", "user=root password=secret dbname=gobackend sslmode=disable")
	if err != nil {
		log.Fatalln(err)
	}

	repo := Repository{db: db}

	app := fiber.New()

	request := make(chan *Request)

	go QuotaWorker(context.Background(), repo, request)

	app.Get("/vouchers/:id", func(c *fiber.Ctx) error {
		id := c.Params("id")

		update := &Request{id: id, result: make(chan int)}
		request <- update
		quota := <-update.result

		return c.JSON(quota)
	})

	log.Fatal(app.Listen(":3000"))
}
