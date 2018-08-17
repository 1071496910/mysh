package db

import (
	"testing"
)

func TestAdd(t *testing.T) {
	passwordGetter := MakePasswordGetter()
	for i := 0; i < 10; i++ {
		password, err := passwordGetter("hpc")
		if err != nil {
			t.Fatal(err)
		}
		t.Log(password)
	}

}
