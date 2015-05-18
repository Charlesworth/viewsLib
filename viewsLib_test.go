package viewLib

import (
	"testing"

	//"github.com/boltdb/bolt"
)

func TestViewInc(t *testing.T) {
	ViewInc("123.456.789.0", "Test Page Name")

	testViews := counter.m["Test Page Name"]
	if testViews != 1 {
		t.Error("Page view counter not incrementing correctly")
	}

	testIP := ips.m["123.456.789.0"]
	if testIP != true {
		t.Error("IP logger map did not record the test IP correctly")
	}
	//t.Error("Error MUTHA-*****R")
}

// func TestInit(t *testing.T) {
// 	Init()
//
// 	//t.Error("Error MUTHA-*****R")
// }
