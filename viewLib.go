package viewLib

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/boltdb/bolt"
)

//counter is an instance of a [pageName]pageView hash map. This is implemented with a
//mutex RW lock to stop goroutine data races
var counter = struct {
	sync.RWMutex
	m map[string]int
}{m: make(map[string]int)}

//ips is an instance of a [ipAdress]bool hash map. We don't care about the bool,
//using a hash map in this case just for the IP Key, as it offers a
//easy implementation on a set with quick insertion. This struct has a
//mutex RW lock to stop goroutine data races
var ips = struct {
	sync.RWMutex
	m map[string]bool
}{m: make(map[string]bool)}

//IPList struct is used to marshal/unmarshal IP visitor data into JSON
//to be sent to current storage
type IPList struct {
	IPs map[string]bool
}

//SavePoint struct is used to marshal/unmarshal pageview data into JSON
//to be sent to current and historic storage
type SavePoint struct {
	PageCounts  map[string]int
	UniqueViews int
}

//init checks checks for previos data, sets up multithreading and then
//initiates the HTTP server. init() does not need to be called, it runs
//automatically when the package is called.
func init() {
	//checks for present DB storage and loads it into memory
	checkForRecords()

	//start goroutine to periodicly write IP and page view sets to disk
	go periodicMemoryWriter()
}

//ViewInc locks the counter and ip set mutexes, writes to both then unlocks
func ViewInc(ip string, page string) {
	log.Println(ip + " requests " + page)

	counter.Lock()
	counter.m[page]++
	counter.Unlock()

	ips.Lock()
	ips.m[ip] = true
	ips.Unlock()
}

//periodicMemoryWriter initiates a BoltDB client, sets up a ticker and
//then wrties the IP and pageView maps to on persistant memory via BoltDB.
//This means that in the highly unlikely ;) case that the program crashes,
//a restart will reload the data and your view count won't vanish.
func periodicMemoryWriter() {
	//start the bolt client
	boltClient, err := bolt.Open("viewCounter.db", 0600, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer boltClient.Close()

	//check and create a bucket in bolt to store the data
	boltClient.Update(func(tx *bolt.Tx) error {
		tx.CreateBucketIfNotExists([]byte("historicData"))
		return nil
	})

	//start a ticker for auto uploading the ips and view count to bolt
	//that triggers every ten minutes
	ticker := time.NewTicker(time.Minute * 10)

	for {

		<-ticker.C
		log.Println("Tick")
		fmt.Println("start:", time.Now())

		date := strconv.Itoa((time.Now().YearDay() * 10000) + time.Now().Year())
		fmt.Println(date)

		counter.RLock()
		ips.RLock()

		m1 := SavePoint{
			PageCounts:  counter.m,
			UniqueViews: len(ips.m),
		}

		m2 := IPList{
			IPs: ips.m,
		}

		counter.RUnlock()
		ips.RUnlock()

		m1json, err := json.Marshal(m1)
		errLog(err)
		m2json, err := json.Marshal(m2)
		errLog(err)
		boltClient.Update(func(tx *bolt.Tx) error {

			err = tx.Bucket([]byte("historicData")).Put([]byte(date), []byte(m1json))
			errLog(err)

			err = tx.Bucket([]byte("historicData")).Put([]byte("current"), []byte(m1json))
			errLog(err)

			err = tx.Bucket([]byte("historicData")).Put([]byte("IPs"), []byte(m2json))
			errLog(err)
			return nil
		})

		fmt.Println("end:", time.Now())

	}
}

//checkForRecords is used to see if a BoltDB database is present in the file system,
//and if it is then to load the IP and pageview sets into program memory.
func checkForRecords() {
	if _, err := os.Stat("viewCounter.db"); err == nil {
		log.Println("viewCount.db database already exists; processing old entries")

		boltClient, err := bolt.Open("viewCounter.db", 0600, nil) //maybe change the 600 to a read only value
		if err != nil {
			log.Fatal(err)
		}
		defer boltClient.Close()

		var b1, b2 []byte
		boltClient.View(func(tx *bolt.Tx) error {
			// Set the value "bar" for the key "foo".
			b1 = tx.Bucket([]byte("historicData")).Get([]byte("current"))
			errLog(err)

			b2 = tx.Bucket([]byte("historicData")).Get([]byte("IPs"))
			errLog(err)

			return nil
		})

		var mjson1 SavePoint
		err = json.Unmarshal(b1, &mjson1)
		errLog(err)

		for k, v := range mjson1.PageCounts {
			counter.m[k] = v
		}

		var mjson2 IPList
		err = json.Unmarshal(b2, &mjson2)
		errLog(err)

		for k := range mjson2.IPs {
			ips.m[k] = true
		}

	} else {
		log.Println("viewCount.db not present; creating database")

	}
}

func errLog(err error) {
	if err != nil {
		log.Print(err)
	}
}
