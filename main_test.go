package main_test

import (
	"fmt"
	"testing"

	"gopkg.in/resty.v1"
)

var client = resty.New().SetRetryCount(0)

func TestMain(t *testing.T) {
	c := make(chan string, 100)

	go func() {
		for i := 0; i < 50; i++ {
			go func(i int) {
				var res interface{}
				client.R().SetResult(&res).Get(fmt.Sprintf("http://localhost:3000/vouchers/7671068b-e450-4aa9-b7a3-cd100203a3c9?p=%d", i))
				c <- fmt.Sprint(i, res)
			}(i)
		}
	}()

	go func() {
		for i := 0; i < 50; i++ {
			go func(i int) {
				var res interface{}
				client.R().SetResult(&res).Get(fmt.Sprintf("http://localhost:3001/vouchers/7671068b-e450-4aa9-b7a3-cd100203a3c9?p=%d", i))
				c <- fmt.Sprint(i, res)
			}(i)
		}
	}()

	for i := 0; i < 100; i++ {
		fmt.Println(<-c)
	}
}
