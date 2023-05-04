package mapreduce

import (
	"database/sql"
	"fmt"
	"hash/fnv"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"runtime"
)

type MapTask struct {
	M, R       int    // total number of map and reduce tasks
	N          int    // map task number, 0-based
	SourceHost string // address of host with map input file
}

type ReduceTask struct {
	M, R        int      // total number of map and reduce tasks
	N           int      // reduce task number, 0-based
	SourceHosts []string // addresses of map workers
}

type Pair struct {
	Key   string
	Value string
}

type Worker struct {
	Address   string
	Available bool
	MapTasks []int
	ReduceTasks []int
}

type Shutdown struct {}

func call(address, method string, request, response interface{}) error {
	client, err := rpc.DialHTTP("tcp", address)
	if err != nil {
		log.Printf("rpc.Dial: %v", err)
		return err
	}
	defer client.Close()

	if err = client.Call(method, request, response); err != nil {
		log.Printf("!!!Client call to the method %s: %v!!!", method, err)
		return err
	}

	return nil
}

// functions for consistent file naming
func mapSourceFile(m int) string       { return fmt.Sprintf("map_%d_source.db", m) }
func mapInputFile(m int) string        { return fmt.Sprintf("map_%d_input.db", m) }
func mapOutputFile(m, r int) string    { return fmt.Sprintf("map_%d_output_%d.db", m, r) }
func reduceInputFile(r int) string     { return fmt.Sprintf("reduce_%d_input.db", r) }
func reduceOutputFile(r int) string    { return fmt.Sprintf("reduce_%d_output.db", r) }
func reducePartialFile(r int) string   { return fmt.Sprintf("reduce_%d_partial.db", r) }
func reduceTempFile(r int) string      { return fmt.Sprintf("reduce_%d_temp.db", r) }
func makeURL(host, file string) string { return fmt.Sprintf("http://%s/data/%s", host, file) }

type Interface interface {
	Map(key, value string, output chan<- Pair) error
	Reduce(key string, values <-chan string, output chan<- Pair) error
}

func (shutdown *Shutdown) Quit(*args, *response) error {
	os.Exit(0)
}

func (task *MapTask) Process(tempdir string, client Interface) error {
	//TESTING
	var count_pairs_pros, count_pairs_gen int

	filename := mapInputFile(task.N)
	fmt.Println("Downloading file in Map Process: " + filename)
	download("http://localhost:8080/data/"+filename, filename)

	db, err := openDatabase(filename)
	if err != nil {
		return err
	}
	defer db.Close()

	var output_dbs []*sql.DB
	var outputFiles []string
	for i := 0; i < task.R; i++ {
		outputFile := tempdir + mapOutputFile(task.N, i)
		outputFiles = append(outputFiles, outputFile)
		createDatabase(outputFile)
		odb, err := openDatabase(outputFile)
		if err != nil {
			return err
		}
		output_dbs = append(output_dbs, odb)
		defer output_dbs[i].Close()
		defer fmt.Printf("Closing output_dbs[%v]", i)
	}

	rows, err := db.Query("select key, value from pairs")
	if err != nil {
		log.Printf("MapTask.Process() rows: error in select query from database to split: %v", err)
		return err
	}

	//stmts := [3]*sql.Stmt{}
	for rows.Next() {
		output := make(chan Pair, 10000)
		count_pairs_pros += 1
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			log.Printf("error scanning row value: %v", err)
			return err
		}

		go client.Map(key, value, output)

		for pair := range output {
			count_pairs_gen += 1
			hash := fnv.New32() // from the stdlib package hash/fnv
			hash.Write([]byte(pair.Key))
			r := int(hash.Sum32() % uint32(task.R))
			_, err = output_dbs[r].Exec(fmt.Sprintf("insert into pairs (key, value) values (%q, %q)", pair.Key, pair.Value))
			if err != nil {
				log.Printf("Error in output_dbs[%v].Prepare(): %v", r, err)
			}
		}
	}
	if err := rows.Err(); err != nil {
		log.Printf("db error iterating over inputs: %v", err)
		return err
	}

	// for _, stmt := range stmts {
	// 	fmt.Println("Calling stmt.Query(1)")
	// 	stmt.Query(1)
	// }

	fmt.Println("Prcessed", count_pairs_pros, "pairs and generated", count_pairs_gen, "pairs")

	return nil
}

func (task *ReduceTask) Process(tempdir string, client Interface) error {
	//TESTING
	var count_keys, count_values, count_pairs int

	//create input database by merging all appropriate map output databases
	reduce_input, err := mergeDatabases(task.SourceHosts, reduceInputFile(task.N), "temp") //not sure about tempdir, and SourceHosts assumes pre-assigned
	if err != nil {
		log.Printf("Error in merging input files for reduce task", task.N)
		return err
	}
	fmt.Println("Merged databases")

	//create (and open) the output database
	outputFile := tempdir + reduceOutputFile(task.N)
	createDatabase(outputFile)
	odb, err := openDatabase(outputFile)
	if err != nil {
		return err
	}

	//process all pairs in correct order
	rows, err := reduce_input.Query("select key, value from pairs order by key, value")
	if err != nil {
		log.Printf("error in select query from database to split: %v", err)
		return err
	}

	output := make(chan Pair, 100)
	values := make(chan string, 100)
	var cur_key string
	var pair Pair
	//var stmt *sql.Stmt
	for rows.Next() {
		//fmt.Println("Scanning next row")
		count_values += 1
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			log.Printf("error scanning row value: %v", err)
			return err
		}
		if cur_key != key {
			fmt.Printf("cur_key != key: %v != %v \n", cur_key, key)
			count_keys += 1
			close(values)
			if count_pairs != 0 {
				pair = <-output
				_, err = odb.Exec(fmt.Sprintf("insert into pairs (key, value) values (%q, %q)", pair.Key, pair.Value))
				if err != nil {
					log.Printf("Reduce Process odb.Exec() threw an error: ", err)
				}
			}
			count_pairs += 1
			output = make(chan Pair, 100)
			values = make(chan string, 100)
			cur_key = key
			go client.Reduce(cur_key, values, output)
		}

		//fmt.Println("Sent value", value)
		values <- value
	}
	if err := rows.Err(); err != nil {
		log.Printf("db error iterating over inputs: %v", err)
		return err
	}
	fmt.Println("Finished reduce rows loop")
	// stmt.Query(1)
	close(values)
	pair = <-output
	odb.Exec(fmt.Sprintf("insert into pairs (key, value) values (%v, %v)", pair.Key, pair.Value))

	fmt.Println("Had", count_keys, "with", count_values, "and", count_pairs+1, "pairs")

	return nil
}

func master(client Interface) error {

	runtime.GOMAXPROCS(1)

	//startup the master server
	fmt.Println("Server started")
	address := "localhost:8080"
	tempdir := "./tmp" + "8080"
	go func() {
		http.Handle("/data/", http.StripPrefix("/data", http.FileServer(http.Dir(tempdir))))
		if err := http.ListenAndServe(address, nil); err != nil {
			log.Printf("Error in HTTP server for %s: %v", address, err)
		}
	}()
	defer os.RemoveAll(tempdir)

	//download("http://localhost:8080/data/austen.db", "austen.db")
	fmt.Println("Splitting austen.db into:")
	var MapPaths []string
	for i := 0; i < 9; i++ {
		MapPaths = append(MapPaths, tempdir+mapInputFile(i))
		fmt.Println("	", tempdir+mapInputFile(i))
	}
	splitDatabase("./data/austen.db", MapPaths)

	//create maptasks
	m := 9
	r := 3
	mapTasks := []*MapTask{}
	for i := 0; i < 9; i++ {
		mt := new(MapTask)
		mt.M = m
		mt.R = r
		mt.N = i
		mt.SourceHost = mapInputFile(i)
		mapTasks = append(mapTasks, mt)
	}

	reduceTasks := []*ReduceTask{}
	for i := 0; i < r; i++ {
		fmt.Println("Creating reduce task", i)
		rt := new(ReduceTask)
		rt.M = m
		rt.R = r
		rt.N = i
		reduceTasks = append(reduceTasks, rt)
	}

	//workers(predetermined addresses + availbility for tasks)
	worker_1 := Worker{Address: "localhost:8081",
		Available: true}

	worker_2 := Worker{Address: "localhost:8082",
		Available: true}

	worker_3 := Worker{Address: "localhost:8083",
		Available: true}

	workers := []Worker{worker_1, worker_2, worker_3}

	//wait until the workers have been started up
	var ready string
	fmt.Print("press enter when the workers are ready")
	fmt.scan(&ready) //discard ready, dont need the value

	for mt := range mapTasks {
		for i:=0; true; i++ {
			worker = workers[i]
			if worker.Available {
				worker.Available = false
				worker.MapTasks = append(worker.MapTasks, mt.N)
				go func() {
						var response []string
						if err := call(worker.Address, "MapTask.Process", &mt, &response); err != nil {
							log.Printf("error with call to MapTask.Process")
						} 
						worker.Available = true
				}
				for rt := range reduceTasks {
					rt.SourceHosts = append(rt.SourceHosts, worker.Address + "/data/" + mapOutputFile(mt.N, rt.N))
				}
				break
			}
			if i == 2 {
				i = -1
			}
		}
	}

	for true {
		if workers[0].Available && workers[1].Available && workers[2].Available {
			break
		}
	}

	var reduceHosts []string
	for rt := range reduceTasks {
		for i:=0; true; i++ {
			worker = workers[i]
			if worker.Available {
				worker.Available = false
				go func() {
						var response []string
						if err := call(worker.Address, "ReduceTask.Process", &rt, &response); err != nil {
							log.Printf("error with call to ReduceTask.Process")
						} 
						worker.Available = true
				}
				reduceHosts = append(reduceHosts, worker.Address + "/data/" + reduceOutputFile(rt.N))
				break
			}
			if i == 2 {
				i = -1
			}
		}
	}
	
	mergeDatabases(reduceHosts, "final.db", "temp")

}

func worker(client Interface) error {
	runtime.GOMAXPROCS(1)

	var port string
	fmt.Print("Port Number? ")
	fmt.Scan(&port)

	//startup the master server
	fmt.Println("Server started")

	mt := new(MapTask)
	rt := new(ReduceTask)
	quit := new(Shutdown)
	rpc.Register(quit)
	rpc.Register(mt)
	rpc.Register(rt)
	address := "localhost:" + port
	tempdir := "./tmp" + port

	http.Handle("/data/", http.StripPrefix("/data", http.FileServer(http.Dir(tempdir))))
	if err := http.ListenAndServe(address, nil); err != nil {
		log.Printf("Error in HTTP server for %s: %v", address, err)
	}

	defer os.RemoveAll(tempdir)
	return nil
}

func Start(client Interface) error {
	var _type string
	fmt.Print("master or worker? ")
	fmt.Scan(&_type)
	switch _type {
	case "worker":
		worker(client)
	case "master":
		master(client)
	}
	return nil
}

// type Client struct{}

// func (c Client) Map(key, value string, output chan<- Pair) error {
// 	defer close(output)
// 	lst := strings.Fields(value)
// 	for _, elt := range lst {
// 		word := strings.Map(func(r rune) rune {
// 			if unicode.IsLetter(r) || unicode.IsDigit(r) {
// 				return unicode.ToLower(r)
// 			}
// 			return -1
// 		}, elt)
// 		if len(word) > 0 {
// 			output <- Pair{Key: word, Value: "1"}
// 		}
// 	}
// 	return nil
// }

// func (c Client) Reduce(key string, values <-chan string, output chan<- Pair) error {
// 	defer close(output)
// 	count := 0
// 	for v := range values {
// 		//fmt.Println("Recieved value", v)
// 		i, err := strconv.Atoi(v)
// 		if err != nil {
// 			return err
// 		}
// 		count += i
// 	}
// 	p := Pair{Key: key, Value: strconv.Itoa(count)}
// 	output <- p
// 	return nil
// }
