// SCREENCAST AT: https://share.vidyard.com/watch/C9XjMN8fkoPsAJdpK8S59g?

package main

import (
	"bufio"
	"crypto/sha1"
	"fmt"
	"log"
	"math/big"
	"net"
	"net/rpc"
	"os"
	"strings"
	"time"
)

type Nothing struct{}

type Key string
type NodeAddress string

type handler func(n *Node)
type Server chan handler

type Node struct {
	Address     NodeAddress
	FingerTable []NodeAddress
	Predecessor NodeAddress
	Successors  []NodeAddress
	Next        int

	Bucket map[Key]string
}

func startActor(listenAddress string) Server {
	ch := make(chan handler)
	n := new(Node)
	n.Address = NodeAddress(listenAddress)
	n.FingerTable = make([]NodeAddress, 160)
	n.Bucket = make(map[Key]string)
	n.Predecessor = n.Address
	go func() {
		for f := range ch {
			//fmt.Println("Actor Loop", f)
			f(n)
			//fmt.Println("Actor Loop f(n) done")
		}
	}()
	return ch
}

func hash(elt string) *big.Int {
	hasher := sha1.New()
	hasher.Write([]byte(elt))
	return new(big.Int).SetBytes(hasher.Sum(nil))
}

const keySize = sha1.Size * 8
const numPred = 3

var two = big.NewInt(2)
var hashMod = new(big.Int).Exp(big.NewInt(2), big.NewInt(keySize), nil)

func jump(address string, fingerentry int) *big.Int {
	n := hash(address)
	fingerentryminus1 := big.NewInt(int64(fingerentry) - 1)
	jump := new(big.Int).Exp(two, fingerentryminus1, nil)
	sum := new(big.Int).Add(n, jump)

	return new(big.Int).Mod(sum, hashMod)
}

func between(start, elt, end *big.Int, inclusive bool) bool {
	if end.Cmp(start) > 0 {
		return (start.Cmp(elt) < 0 && elt.Cmp(end) < 0) || (inclusive && elt.Cmp(end) == 0)
	} else {
		return start.Cmp(elt) < 0 || elt.Cmp(end) < 0 || (inclusive && elt.Cmp(end) == 0)
	}
}

func getLocalAddress() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	return localAddr.IP.String()
}

func help() {
	fmt.Println("port <n>) set the port that this node should listen on. By default, this should be port 3410, but users can set it to something else.")
	fmt.Println("This command only works before a ring has been created or joined. After that point, trying to issue this command is an error.")
	fmt.Println("create) Create a new ring.")
	fmt.Println("This command only works before a ring has been created or joined. After that point, trying to issue this command is an error.")
	fmt.Println("join <address>) join an existing ring, one of whose nodes is at the address specified.")
	fmt.Println("This command only works before a ring has been cread or joined. After that point, trying to issue this command is an error.")
	fmt.Println("quit) shut down. This quits and ends the program. If this was the last instance in a ring, the ring is effectively shut down.")
	fmt.Println("If this is not the last instance, it should send all of its data to its immediate successor before quitting. Other than that it is not necessary to notify the rest of the ring when a node shuts down.")
	fmt.Println("\nput <key> <value>) insert the given key and value into the currently active ring. The instance must find the peer that is responsible for the given key using DHT lookup operation, then contact that host directly and send it the key and value to be stored.")
	fmt.Println("putrnadom <n>) randomly generate n keys (and accompanying values) and put each pair into the ring. Useful for debugging.")
	fmt.Println("get <key>) find the given key in the currently active ring. The instance must find the peer that is responsible for the giben key using a DHT lookup operation, then contact that host directly and retrieve the value and display it to the local user.")
	fmt.Println("delete <key>) similar to lookup, but instead of retrieving the value and displaying it, the peer deletes it from the ring.")
	fmt.Println("\ndump) display information about the current node, including the range of keys it is responsible for, its predecessor and succesor links, its finger table, and the actual key/value pairs that is stores.")
	fmt.Println("\nhelp) this displays a list of recognized commands.")
}

func create(listenAddress string) Server {
	s := startActor(listenAddress)
	rpc.Register(s)
	finished := make(chan struct{})
	s <- func(n *Node) {
		n.Successors = []NodeAddress{n.Address}
		finished <- struct{}{}
	}
	<-finished
	listenAddress = net.JoinHostPort("", listenAddress)
	listener, err := net.Listen("tcp", listenAddress)
	if err != nil {
		log.Fatal(err)
	}
	go rpc.Accept(listener)
	fmt.Println("Listening at ", listenAddress)
	s.stabalizeRoutine()
	return s
}

func (s Server) ping(serverAddress string) (int, error) {
	address := ""
	finished := make(chan struct{})
	s <- func(n *Node) {
		address = string(n.Address)
		finished <- struct{}{}
	}
	<-finished
	var response int
	serverAddress = fixAddress(serverAddress)
	if err := call(serverAddress, "Server.Ping", address, &response); err != nil {
		//log.Fatalf("client.Call %v", err)
		return 404, nil
	}
	//fmt.Println("Sent ping and got response: ", response)
	return response, nil
}

func (s Server) Ping(clientAddress string, reply *int) error {
	//fmt.Println("Received ping from port ", clientAddress)
	*reply = 200
	return nil
}

func (s Server) get(key string) error {
	finished := make(chan struct{})
	my_address := ""
	s <- func(n *Node) {
		my_address = string(n.Address)
		finished <- struct{}{}
	}
	<-finished

	var response string
	key_hash := hash(key)
	successor, _ := find(key_hash, my_address)
	if successor == "" {
		fmt.Println("FAILED TO FIND SUCCESSOR OF", key)
	}
	successor = fixAddress(successor)
	if err := call(successor, "Server.Get", key, &response); err != nil {
		log.Fatalf("client.Call %v", err)
	}
	fmt.Println("Recieved", key, "with value", response)

	return nil
}

func (s Server) Get(key string, reply *string) error {
	finished := make(chan struct{})
	s <- func(n *Node) {
		if _, ok := n.Bucket[Key(key)]; ok {
			*reply = n.Bucket[Key(key)]
			//fmt.Println("Sent value: ", *reply)
		} else {
			*reply = "Status Code 404"
			fmt.Println("Key not found: ", key)
		}
		finished <- struct{}{}
	}
	<-finished
	return nil
}

func (s Server) put(key, value string) error {
	finished := make(chan struct{})
	my_address := ""
	s <- func(n *Node) {
		my_address = string(n.Address)
		finished <- struct{}{}
	}
	<-finished

	var response string
	key_hash := hash(key)
	successor, _ := find(key_hash, my_address)
	if successor == "" {
		log.Fatalf("FAILED TO FIND SUCCESSOR OF %v", key)
	}
	key_value := key + " " + value
	successor = fixAddress(successor)
	if err := call(successor, "Server.Put", key_value, &response); err != nil {
		log.Fatalf("client.Call %v", err)
	}
	fmt.Println("Put", key_value, "sent to", successor, "with response", response)

	return nil
}

func (s Server) Put(key_value string, reply *string) error {
	finished := make(chan struct{})
	s <- func(n *Node) {
		content := strings.Split(key_value, " ")
		key := Key(content[0])
		value := content[1]
		n.Bucket[key] = value
		*reply = "Status Code 200"
		fmt.Println("Put key value pair", key_value, "in my bucket")
		finished <- struct{}{}
	}
	<-finished
	return nil
}

func (s Server) delete(key string) error {
	finished := make(chan struct{})
	my_address := ""
	s <- func(n *Node) {
		my_address = string(n.Address)
		finished <- struct{}{}
	}
	<-finished

	var response string
	key_hash := hash(key)
	successor, _ := find(key_hash, my_address)
	if successor == "" {
		fmt.Println("FAILED TO FIND SUCCESSOR OF", key)
	}
	successor = fixAddress(successor)
	if err := call(successor, "Server.Delete", key, &response); err != nil {
		log.Fatalf("client.Call %v", err)
	}
	fmt.Println("Deleted", key, "from node", successor, "with response", response)

	return nil
}

func (s Server) Delete(key string, reply *string) error {
	finished := make(chan struct{})
	s <- func(n *Node) {
		//print(key)
		if _, ok := n.Bucket[Key(key)]; ok {
			delete(n.Bucket, Key(key))
			*reply = "Status Code 200"
			fmt.Println("Deleted key: ", key)
		} else {
			*reply = "Status Code 404"
			fmt.Println("Delete Request: Key not found: ", key)
		}
		finished <- struct{}{}
	}
	<-finished
	return nil
}

func (s Server) dump() {
	finished := make(chan struct{})
	s <- func(n *Node) {
		fmt.Println("\n------Finger Table-------")
		for i, s := range n.FingerTable {
			fmt.Printf("%v: %v\n", i, s)
		}
		fmt.Println("\n------Bucket------")
		for k, v := range n.Bucket {
			fmt.Printf("%-10v %v\n", string(k), v)
		}
		fmt.Println("\n------Successors-------")
		for _, s := range n.Successors {
			fmt.Printf("%v\n", s)
		}
		fmt.Println("\n------Predecessor-------")
		fmt.Println(n.Predecessor)
		fmt.Println()
		finished <- struct{}{}
	}
	<-finished
}

func join(serverAddress, port string) Server {
	s := startActor(port)
	rpc.Register(s)
	finished := make(chan struct{})
	my_address := ""
	s <- func(n *Node) {
		my_address = string(n.Address)
		finished <- struct{}{}
	}
	<-finished

	address_hash := hash(my_address)
	successor, _ := find(address_hash, serverAddress)
	if successor == "" {
		fmt.Println("FAILED TO FIND SUCCESSOR")
	}

	s <- func(n *Node) {
		n.Successors = append(n.Successors, NodeAddress(successor))
		n.Predecessor = n.Address
		fmt.Println("Added", successor, "to this node's successor's")

		//initialize server
		n.Bucket = make(map[Key]string)
		finished <- struct{}{}
	}
	<-finished

	listenAddress := net.JoinHostPort("", my_address)
	listener, err := net.Listen("tcp", listenAddress)
	if err != nil {
		log.Fatal(err)
	}
	go rpc.Accept(listener)
	fmt.Println("Listening at ", listenAddress)

	s.stabalizeRoutine()
	s.getAll(successor)

	return s
}

func fixAddress(addr string) string {
	if strings.HasPrefix(addr, ":") || strings.HasPrefix(addr, "localhost:") {
		return addr
	}
	return ":" + addr
}

func (s Server) putAll(address string, m map[Key]string) error {
	var response map[Key]string
	address = fixAddress(address)
	if err := call(address, "Server.PutAll", m, &response); err != nil {
		fmt.Printf("client.Call %v", err)
		return err
	}
	fmt.Println("Put All request to", address, "with response", response)
	return nil
}

func (s Server) PutAll(m map[Key]string, reply *map[Key]string) error {
	finished := make(chan struct{})
	s <- func(n *Node) {
		for key, value := range m {
			n.Bucket[key] = value
			fmt.Println("Put", key, value, "in my bucket")
		}
		finished <- struct{}{}
	}
	<-finished
	return nil
}

func (s Server) getAll(address string) error {
	finished := make(chan struct{})
	myAddress := ""
	s <- func(n *Node) {
		myAddress = string(n.Address)
		finished <- struct{}{}
	}
	<-finished
	var response map[Key]string
	address = fixAddress(address)
	if err := call(address, "Server.GetAll", myAddress, &response); err != nil {
		fmt.Printf("client.Call %v", err)
		return err
	}
	fmt.Println("Get All request to", address, "with response ", response)
	s <- func(n *Node) {
		for key, value := range response {
			n.Bucket[Key(key)] = value
			fmt.Println("Added", key, value, "to my bucket")
		}
		finished <- struct{}{}
	}
	<-finished
	return nil
}

func (s Server) GetAll(address string, reply *map[Key]string) error {
	finished := make(chan struct{})
	s <- func(n *Node) {
		for key, value := range n.Bucket {
			fmt.Println(address)
			if between(hash(string(n.Address)), hash(string(key)), hash(string(address)), true) {
				(*reply)[key] = value
				delete(n.Bucket, key)
				fmt.Println("Deleted", key, value, "from my bucket")
			}
		}
		finished <- struct{}{}
	}
	<-finished
	return nil
}

func find(id_hash *big.Int, start string) (string, error) {
	found := false
	nextNode := start
	i := 0
	maxSteps := 15
	var err error
	for !found && i < maxSteps {
		//fmt.Println("Not found moving to", nextNode)
		if strings.HasPrefix(nextNode, ":") || strings.HasPrefix(nextNode, "localhost:") {
			nextNode, found, err = findSuccessor(nextNode, id_hash)
			if err != nil {
				return "", err
			}
		} else {
			nextNode, found, err = findSuccessor(":"+nextNode, id_hash)
			if err != nil {
				return "", err
			}
		}
		i += 1
	}
	if found {
		return nextNode, nil
	}
	log.Fatalf("FAILED TO FIND SUCCESSOR")
	return "", nil
}

func findSuccessor(address string, id_hash *big.Int) (string, bool, error) {
	var response []string
	address = fixAddress(address)
	if err := call(address, "Server.FindSuccessor", id_hash, &response); err != nil {
		//print("response: ", response)
		fmt.Printf("client.Call %v", err)
		return "", false, err
	}
	//fmt.Println("Find successor request with response ", response)
	if response[1] == "true" {
		//fmt.Println("FindSuccessor replying with ", response[0], "true")
		return response[0], true, nil
	}
	//fmt.Println("FindSuccessor replying with ", response[0], "false")
	return response[0], false, nil
}

func (s Server) FindSuccessor(id_hash *big.Int, reply *[]string) error {
	finished := make(chan struct{})
	myAddress := ""
	successor := ""
	s <- func(n *Node) {
		myAddress = string(n.Address)
		successor = string(n.Successors[0])
		if successor == "" {
			print("successor == ''")
		}
		finished <- struct{}{}
	}
	<-finished
	if between(hash(myAddress), id_hash, hash(successor), true) {
		*reply = []string{successor, "true"}
		//fmt.Println("FindSuccessor replying with ", successor, "true")
	} else {
		cpn := s.closestPrecedingNode(id_hash)
		//fmt.Println("cpn was called")
		*reply = []string{cpn, "false"}
		//fmt.Println("FindSuccessor replying with ", cpn, "false (cpn was called from", myAddress)
	}
	return nil
}

func (s Server) closestPrecedingNode(id_hash *big.Int) string {
	returnValue := ""
	finished := make(chan struct{})
	s <- func(n *Node) {
		for i := len(n.FingerTable) - 1; i >= 0; i-- {
			//fmt.Println("ClosestPrecedingNode looping through finger table: ", i, n.FingerTable[i])
			if n.FingerTable[i] == "" {
				//fmt.Println("ClosestPrecedingNode skipping: ", i, n.FingerTable[i])
				continue
			}
			if between(hash(string(n.Address)), hash(string(n.FingerTable[i])), id_hash, false) {
				//fmt.Println("ClosestPrecedingNode returning: ", i, n.FingerTable[i])
				returnValue = string(n.FingerTable[i])
				finished <- struct{}{}
				return
			}
		}
		//fmt.Println("ClosestPrecedingNode returning successor: ", n.Successors[0])
		returnValue = string(n.Successors[0])
		finished <- struct{}{}
	}
	<-finished
	return returnValue
}

func (s Server) stabalize() {
	successors_before := make([]NodeAddress, 5)
	successor := ""
	my_address := ""
	my_successors := []NodeAddress{}
	finished := make(chan struct{})
	s <- func(n *Node) {
		//fmt.Println("Stabalize() started")
		successors_before = n.Successors

		//fmt.Println("My successors are ", n.Successors)
		successor = string(n.Successors[0])
		my_address = string(n.Address)
		my_successors = n.Successors
		finished <- struct{}{}
	}
	<-finished

	//get successor's predecessor
	x, err := getPredecessor(":" + successor)
	//fmt.Println("Successor's Predecessor", x)

	//if contact to successor failed assume the successor went down and remove it from the successor list
	//The next successor becomes your successor. If there are none, then your successor is yourself
	if err != nil {
		s.handleSuccessorNotFound()
		s <- func(n *Node) {
			successor = string(n.Successors[0])
			my_successors = n.Successors
			finished <- struct{}{}
		}
		<-finished
		return
	}

	//check if successor's predecessor is between me and my successor
	if between(hash(my_address), hash(x), hash(successor), false) && x != "" {
		//fmt.Println(x, "is between me and my successor")
		successor = x
		my_successors = []NodeAddress{NodeAddress(successor), my_successors[0]}
	} else {
		//fmt.Println(x, "is not between me and my successor")
		my_successors = []NodeAddress{my_successors[0]}
		//fmt.Println("My successors are now ", n.Successors[0])
	}

	//fmt.Println("successor:", successor, "me:", my_address)
	_, err = notify(":"+successor, my_address)
	if err != nil {
		s.handleSuccessorNotFound()
		s <- func(n *Node) {
			successor = string(n.Successors[0])
			finished <- struct{}{}
		}
		<-finished
		//fmt.Println("successor:", successor, "me:", my_address)
		notify(":"+successor, my_address)
	}

	//update your successors list
	ss, err := getSuccessors(":" + successor)
	if err != nil {
		return
	}

	//fmt.Println("Successor's successors", ss)
	for _, address := range ss {
		if len(my_successors) >= numPred {
			break
		}
		//fmt.Println("len(my_successors): ", len(my_successors))
		my_successors = append(my_successors, address)
	}
	s <- func(n *Node) {
		n.Successors = my_successors
		finished <- struct{}{}
	}
	<-finished

	//Check if successors are new
	equal := true
	if len(my_successors) != len(successors_before) {
		equal = false
	} else {
		for i, v := range my_successors {
			if v != successors_before[i] {
				equal = false
				break
			}
		}
	}
	if !equal {
		fmt.Println("New successors:", my_successors)
	}
	//fmt.Println("Stabalize() finished")
}

func (s Server) stabalizeRoutine() {
	go func() {
		for {
			//fmt.Println("StabalizeRoutine looping")
			time.Sleep(time.Second / 3)
			s.stabalize()
			time.Sleep(time.Second / 3)
			s.checkPredecessor()
			time.Sleep(time.Second / 3)
			s.fixFingers()
		}
	}()
	//fmt.Println("StabalizeRoutine started and function finished")
}

func (s Server) handleSuccessorNotFound() {
	finished := make(chan struct{})
	s <- func(n *Node) {
		successor := n.Successors[0]
		print("\n")
		fmt.Println("Could not contact", successor, "removing", successor, "from my successors")
		print("\n")
		n.Successors = n.Successors[1:len(n.Successors)]
		if len(n.Successors) == 0 {
			n.Successors = append(n.Successors, n.Address)
		}
		fmt.Println("Successors are now", n.Successors)
		finished <- struct{}{}
	}
	<-finished
}

func notify(address string, nodeToNotify string) (string, error) {
	var response string
	if err := call(address, "Server.Notify", nodeToNotify, &response); err != nil {
		return response, err
		//log.Fatalf("client.Call %v", err)
	}
	//fmt.Println("notify request with response ", response)
	return response, nil
}

func (s Server) Notify(address string, reply *string) error {
	finished := make(chan struct{})
	s <- func(n *Node) {
		//fmt.Println("-----NOTIFY------")
		if n.Predecessor == "" {
			fmt.Println("Predecessor is empty. Setting my predecessor to", address)
			n.Predecessor = NodeAddress(address)
		}
		if between(hash(string(n.Predecessor)), hash(address), hash(string(n.Address)), false) {
			//fmt.Println(address, "is between", n.Predecessor, "and", n.Address)
			if NodeAddress(address) != n.Predecessor {
				fmt.Println("Setting my predecessor to", address)
				n.Predecessor = NodeAddress(address)
			}
		}
		finished <- struct{}{}
	}
	<-finished
	return nil
}

func getPredecessor(address string) (string, error) {
	var response string
	address = fixAddress(address)
	if err := call(address, "Server.GetPredecessor", "", &response); err != nil {
		//log.Fatalf("client.Call %v", err)
		return "", err
	}
	//fmt.Println("Get Predecessor request with response ", response)
	return response, nil
}

func (s Server) GetPredecessor(key string, reply *string) error {
	finished := make(chan struct{})
	s <- func(n *Node) {
		*reply = string(n.Predecessor)
		//fmt.Println("Sent Predecessor: ", *reply)
		finished <- struct{}{}
	}
	<-finished
	return nil
}

func getSuccessors(address string) ([]NodeAddress, error) {
	var response []NodeAddress
	if err := call(address, "Server.GetSuccessors", "", &response); err != nil {
		return response, err
		//log.Fatalf("client.Call %v", err)
	}
	//fmt.Println("Get Successors with response ", response)
	return response, nil
}

func (s Server) GetSuccessors(key string, reply *[]NodeAddress) error {
	finished := make(chan struct{})
	s <- func(n *Node) {
		*reply = n.Successors
		//fmt.Println("Sent Successors: ", *reply)
		finished <- struct{}{}
	}
	<-finished
	return nil
}

func (s Server) checkPredecessor() {
	//fmt.Println("checkPredecessor() starting")
	finished := make(chan struct{})
	pred := ""
	s <- func(n *Node) {
		pred = string(n.Predecessor)
		finished <- struct{}{}
	}
	<-finished
	if pred == "" {
		return
	}
	if response, _ := s.ping(pred); response != 200 {
		fmt.Println("Failed to contact predecessor. Setting predecessor to '' (empty string/NodeAddress)")
		s <- func(n *Node) {
			n.Predecessor = ""
			finished <- struct{}{}
		}
		<-finished
	}
	//fmt.Println("checkPredecessor() finished")
}

func (s Server) fixFingers() {
	//fmt.Println("fixFingers() starting")
	finished := make(chan struct{})
	current := ""
	my_address := ""
	valueForUpdate := ""
	next := 0
	s <- func(n *Node) {
		//fmt.Println("Fixing finger table entry ", n.Next)
		current = string(n.FingerTable[n.Next])
		my_address = string(n.Address)
		next = n.Next
		finished <- struct{}{}
	}
	<-finished

	//fmt.Println("before jump")
	j := jump(my_address, next)
	//mt.Println("before find")
	valueForUpdate, err := find(j, my_address)
	if err != nil {
		return
	}
	//fmt.Println("valueForUpdate: ", valueForUpdate, "current: ", current)

	if valueForUpdate != current {
		//fmt.Println("updating table", next+1)
		s <- func(n *Node) {
			n.FingerTable[n.Next] = NodeAddress(valueForUpdate)
			fmt.Println("Changed my fingertable[", n.Next, "] to", n.FingerTable[n.Next])
			finished <- struct{}{}
		}
		<-finished
	}

	start, err := getPredecessor(valueForUpdate)
	//fmt.Println("gotPredecessor")
	if err != nil {
		fmt.Println("Error in getPredecessor in fixFingers()")
		return
	}
	end := string(valueForUpdate)
	s <- func(n *Node) {
		n.Next += 1
		if n.Next >= 160 {
			n.Next = 0
		} else {
			for between(hash(start), jump(string(n.Address), n.Next), hash(end), false) && n.Next < 160 && n.Next > 0 {
				//fmt.Println("jump(" + string(n.Address) + ", " + fmt.Sprint(n.Next+1) + ") was in between " + start + " and " + end)

				current := n.FingerTable[n.Next]

				n.FingerTable[n.Next] = n.FingerTable[n.Next-1]

				if n.FingerTable[n.Next] != current {
					fmt.Println("Changed my fingertable[", n.Next, "] to", n.FingerTable[n.Next])
				}

				n.Next += 1
			}
		}

		if n.Next >= 160 {
			n.Next = 0
		}
		finished <- struct{}{}
	}
	<-finished
	//fmt.Println("fixFingers() finished")
}

func call(serverAddress string, method string, request interface{}, response interface{}) error {
	if strings.HasPrefix(serverAddress, ":") {
		serverAddress = "localhost" + serverAddress
	}
	client, err := rpc.Dial("tcp", serverAddress)
	if err != nil {
		fmt.Println("------NETWORK ERROR------")
		fmt.Println("address: ", serverAddress)
		fmt.Println("method: ", method)
		return err
		//log.Fatalf("rpc.Dial: %v", err)
	}
	defer client.Close()

	if err = client.Call(method, request, response); err != nil {
		log.Fatalf("client.Call %s: %v", method, err)
	}
	return nil
}

func main() {
	port := "3410"
	in_ring := false
	s := make(Server)

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		//fmt.Println("Scanning line")
		msg := scanner.Text()
		command := strings.Split(msg, " ")
		switch command[0] {
		case "help":
			help()
		case "port":
			if len(command) > 1 {
				if !in_ring {
					port = command[1]
					fmt.Println("Changed my address to", port)
				} else {
					fmt.Println("Cannot change port once node is part of the ring!")
				}
			} else {
				fmt.Println("Incorrect Usage of port: port <n>")
			}
		case "create":
			if !in_ring {
				s = create(port)
				in_ring = true
			} else {
				fmt.Println("Cannot create a new ring. You are already in a ring!")
			}
		case "join":
			if !in_ring {
				if len(command) > 1 {
					s = join(command[1], port)
					in_ring = true
				} else {
					fmt.Println("Incorrect Usage of join: join <address>")
				}
			} else {
				fmt.Println("Connot join a ring. You are already in a ring!")
			}
		case "quit":
			successor := ""
			bucket := make(map[Key]string)
			finished := make(chan struct{})
			s <- func(n *Node) {
				n.Address = NodeAddress(port)
				successor = string(n.Successors[0])
				bucket = n.Bucket
				finished <- struct{}{}
			}
			<-finished
			s.putAll(successor, bucket)
			return
		case "put":
			if len(command) > 2 {
				s.put(command[1], command[2])
			} else {
				fmt.Println("Incorrect Usage of put: put <key> <value>")
			}
		case "putrandom":
			continue
		case "get":
			if len(command) > 1 {
				s.get(command[1])
			} else {
				fmt.Println("Incorrect Usage of get: get <key>")
			}
		case "delete":
			if len(command) > 1 {
				s.delete(command[1])
			} else {
				fmt.Println("Incorrect Usage of delete: delete <key>")
			}
		case "dump":
			//fmt.Println("Dump about to be called")
			s.dump()
			//fmt.Println("Dump Called")
		case "ping":
			if len(command) > 1 {
				s.ping(command[1])
			} else {
				fmt.Println("Incorrect Usage of ping: ping <n>")
			}
		case "":
			continue
		default:
			fmt.Println("Unrecognized command!!!!!")
			help()
		}

	}
	if err := scanner.Err(); err != nil {
		log.Fatal("error scanning input:", err)
	}
}
