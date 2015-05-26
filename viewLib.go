package viewLib

import (
	"encoding/json"
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/boltdb/bolt"
)

//Counter is an instance of a [pageName]pageView hash map. This is implemented
//with a mutex RW lock for concurrent R/W safety
var Counter = struct {
	sync.RWMutex
	M map[string]int
}{M: make(map[string]int)}

//IPs is an instance of a [ipAdress]bool hash map. hash offers a
//easy implementation of a set with quick insertion. This is implemented
//with a mutex RW lock for concurrent R/W safety
var IPs = struct {
	sync.RWMutex
	M map[string]bool
}{M: make(map[string]bool)}

//RefreshTime is how often the page views and IPs are saved to disk, set at a
//default of 1 minute
var SaveDuration = time.Minute * 1

//IPList struct is used to marshal/unmarshal IP visitor data into JSON
//for disk storage
type IPList struct {
	IPs map[string]bool
}

//SavePoint struct is used to marshal/unmarshal pageview data into JSON
//for disk storage
type SavePoint struct {
	PageCounts  map[string]int
	UniqueViews int
}

//init checks checks for previos data and then initiates the HTTP server.
//init() does not need to be called, it runs on startup automatically.
func init() {
	checkForRecords()
	go periodicMemoryWriter()
}

//ViewInc locks the Counter and ip set mutexes, writes to both then unlocks
func ViewInc(ip string, page string) {
	log.Println(ip + " requests " + page)

	Counter.Lock()
	Counter.M[page]++
	Counter.Unlock()

	IPs.Lock()
	IPs.M[ip] = true
	IPs.Unlock()
}

//AddPage adds a new page to the Counter with 0 views
func AddPage(page string) {
	Counter.Lock()
	Counter.M[page] = 0
	Counter.Unlock()
}

//DeletePage deletes the page and its views from the counter
func DeletePage(page string) {
	Counter.Lock()
	delete(Counter.M, page)
	Counter.Unlock()
}

//GetPageViews returns a boolean to indicate if a page is present in the
//counter. If it is it also returns the count, else count = 0
func GetPageViews(page string) (count int, exists bool) {
	Counter.RLock()
	count, exists = Counter.M[page]
	Counter.RUnlock()
	return
}

//GetNumberOfUniqueIPs returns the number of unique IPs
func GetNumberOfUniqueIPs(page string) (numberOfUniqueIPs int) {
	IPs.RLock()
	numberOfUniqueIPs = len(IPs.M)
	IPs.RUnlock()
	return
}

//periodicMemoryWriter initiates a BoltDB client, sets up a ticker and
//then wrties the maps to disk. Should be called as a go routine.
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

	//start a ticker for period between disk writes
	ticker := time.NewTicker(SaveDuration)

	for {

		<-ticker.C
		//Debug fmt.Println("Save start time: ", time.Now())

		//The date is made of the day number concatinated with the year, e.g. the
		//05/01/2015 would be 52015
		date := strconv.Itoa((time.Now().YearDay() * 10000) + time.Now().Year())

		Counter.RLock()
		IPs.RLock()

		m1 := SavePoint{
			PageCounts:  Counter.M,
			UniqueViews: len(IPs.M),
		}

		m2 := IPList{
			IPs: IPs.M,
		}

		Counter.RUnlock()
		IPs.RUnlock()

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
		//Debug fmt.Println("Save finish time: ", time.Now())
	}
}

//checkForRecords is used to see if a BoltDB database is present in the file system,
//and if it is then to load the IP and pageview sets into program memory.
func checkForRecords() {
	if _, err := os.Stat("viewCounter.db"); err == nil {
		log.Println("viewCount.db database already exists; processing old entries")

		boltClient, err := bolt.Open("viewCounter.db", 0600, nil)
		if err != nil {
			log.Fatal(err)
		}
		defer boltClient.Close()

		var b1, b2 []byte
		boltClient.View(func(tx *bolt.Tx) error {
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
			Counter.M[k] = v
		}

		var mjson2 IPList
		err = json.Unmarshal(b2, &mjson2)
		errLog(err)

		for k := range mjson2.IPs {
			IPs.M[k] = true
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
