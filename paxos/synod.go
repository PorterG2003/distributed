package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"
)

type State struct {
	nodes    []Node
	messages map[Key]Packet
}

const (
	MsgPrepareRequest = iota
	MsgPrepareResponse
	MsgAcceptRequest
	MsgAcceptResponse
	MsgDecideRequest
)

type Key struct {
	Type   int
	Time   int
	Target int
}

type Packet struct {
	Sequence int
	From     int
	Value    int
}

type Node struct {
	ID              int
	HighestPrepare  int
	HighestAccept   int
	PromiseSequence int
	AcceptSequence  int
	ChosenValue     int

	ProposeSequence int
	ProposeValue    int
	PromiseVotes    []PromiseVote
	//AcceptVotes     []AcceptVote

	majority int
}

type PromiseVote struct {
	Sender         int
	Okay           bool
	AcceptValue    int
	AcceptSequence int
}

//type AcceptVote []int {
//}

// good
func (s *State) TryInitialize(line string) bool {
	var size int
	n, err := fmt.Sscanf(line, "initialize %d nodes\n", &size)
	if err != nil || n != 1 {
		return false
	}

	if size > 0 && size < 10 {
		for i := 0; i < size; i++ {
			var n Node
			n.ID = i + 1
			n.ProposeSequence = 5000 + n.ID
			n.HighestAccept = -10
			n.majority = size / 2
			s.nodes = append(s.nodes, n)
		}
	} else {
		return false
	}
	fmt.Printf("--> initialized %d nodes\n", size)
	return true
}

// good
func (s *State) TrySendPrepare(line string) bool {
	var t1 int
	var id int
	var n1 Node
	n, err := fmt.Sscanf(line, "at %d send prepare request from %d\n", &t1, &id)
	if err != nil || n != 2 || id < 1 || id > 9 {
		return false
	}

	n1 = s.nodes[id-1]
	for _, n2 := range s.nodes {
		if n2.ID == id {
			continue
		}
		var k Key
		k.Type = MsgPrepareRequest
		k.Time = t1
		k.Target = n2.ID
		s.messages[k] = Packet{Sequence: n1.ProposeSequence, From: id}
	}
	fmt.Printf("--> sent prepare requests to all nodes from %d with sequence %d\n", n1.ID, n1.ProposeSequence)
	return true
}

// good I think
func (s *State) TryDeliverPrepareRequest(line string) bool {
	var t1 int
	var t2 int
	var id int
	n, err := fmt.Sscanf(line, "at %d deliver prepare request message to %d from time %d\n", &t2, &id, &t1)
	if err != nil || n != 3 {
		return false
	}

	var fromKey Key
	for key := range s.messages {
		if key.Time == t1 {
			fromKey = key
		}
	}
	from := s.messages[fromKey].From
	var proposeKey = Key{Type: MsgPrepareRequest, Time: t1, Target: id}
	pack := s.messages[proposeKey]
	if pack.Sequence > s.nodes[id-1].ProposeSequence {
		s.nodes[id-1].ProposeSequence = pack.Sequence
	}
	if s.nodes[id-1].ProposeSequence > s.nodes[id-1].PromiseSequence {
		fmt.Printf("--> prepare request from %d sequence %d accepted by %d with no value\n", from, s.nodes[id-1].ProposeSequence, id)
	} else {
		fmt.Printf("--> prepare request from %d sequence %d denied by %d with no value\n", from, s.nodes[id-1].ProposeSequence, id)
	}
	//fmt.Print(MsgPrepareResponse, t2, id)
	//fmt.Print(s.proposer, s.nodes[id-1].ProposeSequence)
	var newKey = Key{Type: MsgPrepareResponse, Time: t2, Target: id}
	s.messages[newKey] = Packet{From: 1, Sequence: s.nodes[id-1].ProposeSequence}
	return true
}

func (s *State) TryDeliverPrepareResponse(line string) bool {
	var t1 int
	var id int
	var t2 int
	var n1 Node
	n, err := fmt.Sscanf(line, "at %d deliver prepare response message to %d from time %d\n", &t2, &id, &t1)
	if err != nil || n != 3 {
		return false
	}

	n1 = s.nodes[id-1]
	var k Key
	for key := range s.messages {
		if key.Time == t1 {
			k = key
		}
	}

	proposer := id
	//fmt.Print(proposer)
	from := k.Target
	pack := s.messages[k]
	for key := range s.messages {
		if key.Type == MsgAcceptRequest && key.Target == id && s.messages[key].From == from && s.messages[key].Sequence == pack.Sequence {
			fmt.Printf("--> prepare response from %d sequence %d ignored as a duplicate by %d\n", from, pack.Sequence, id)
			return true
		}
	}
	votes := 0
	for _, value := range s.nodes[proposer-1].PromiseVotes {
		if value.Okay {
			votes++
		}
	}
	k = Key{Type: MsgAcceptRequest, Target: id, Time: t2}
	s.messages[k] = Packet{From: from, Sequence: pack.Sequence}
	//fmt.Print(MsgPrepareResponse, t1, id)
	//fmt.Print(k.Target)
	//fmt.Print(n1.majority)
	//fmt.Print(votes)
	if pack.Sequence < s.nodes[id-1].AcceptSequence {
		s.nodes[proposer-1].PromiseVotes = append(s.nodes[id-1].PromiseVotes, PromiseVote{Sender: from, AcceptSequence: pack.Sequence, Okay: false})
		fmt.Printf("--> prepare request from %d sequence %d denied by %d with no value\n", from, pack.Sequence, id)
	}
	if pack.Sequence > s.nodes[id-1].AcceptSequence {
		s.nodes[proposer-1].PromiseVotes = append(s.nodes[id-1].PromiseVotes, PromiseVote{Sender: from, AcceptSequence: pack.Sequence, Okay: true})
		fmt.Printf("--> positive prepare response from %d sequence %d recorded by %d with no value\n", from, pack.Sequence, id)
	}
	if votes > n1.majority {
		s.nodes[proposer-1].ChosenValue = proposer * 11111
		fmt.Printf("--> prepare round successful: %d proposing its own value %d\n", proposer, s.nodes[proposer-1].ChosenValue)
	}
	//fmt.Print(s.messages[k])
	return true
}

func (s *State) TryDeliverAcceptRequest(line string) bool {
	var t1 int
	var id int
	var t2 int
	n, err := fmt.Sscanf(line, "at %d deliver accept request message to %d from time %d\n", &t2, &id, &t1)
	if err != nil || n != 3 {
		return false
	}

	var k Key
	for key := range s.messages {
		if key.Time == t1 {
			k = key
		}
	}
	n1 := s.nodes[s.messages[k].From-1]
	n2 := s.nodes[id-1]
	var k1 Key
	k1.Type = MsgAcceptRequest
	k1.Time = t2
	k1.Target = id
	s.messages[k] = Packet{Sequence: n1.ProposeSequence, From: id, Value: n1.ChosenValue}
	if n2.ProposeSequence >= n1.ProposeSequence {
		s.nodes[id-1].AcceptSequence = n1.ProposeSequence
		n2.ChosenValue = n1.ChosenValue
		fmt.Printf("--> sent accept requests to all nodes from %d with value %d sequence %d\n", n1.ID, n1.ChosenValue, n1.ProposeSequence)
	}
	return true
}

func (s *State) TryDeliverAcceptResponse(line string) bool {
	var t1 int
	var node int
	var t2 int
	n, err := fmt.Sscanf(line, "at %d deliver accept response message to %d from time %d\n", &t2, &node, &t1)
	if err != nil || n != 3 {
		return false
	}
	var k Key
	k.Type = 0
	k.Time = t1
	//s.messages[k] = fmt.Sprintf("--> sent prepare requests to all nodes from %d with sequence %d\n", node, t1)
	fmt.Print(s.messages[k])
	return true
}

func (s *State) TryDeliverDecideRequest(line string) bool {
	var t1 int
	var node int
	var t2 int
	n, err := fmt.Sscanf(line, "at %d deliver decide request message to %d from time %d\n", &t2, &node, &t1)
	if err != nil || n != 3 {
		return false
	}
	var k Key
	k.Type = 0
	k.Time = t1
	//s.messages[k] = fmt.Sprintf("--> sent prepare requests to all nodes from %d with sequence %d\n", node, t1)
	fmt.Print(s.messages[k])
	return true
}

// your code goes here
func main() {
	state := &State{messages: make(map[Key]Packet)}
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		// trim comments
		if i := strings.Index(line, "//"); i >= 0 {
			line = line[:i]
		}
		// ignore empty/comment-only lines
		if len(strings.TrimSpace(line)) == 0 {
			continue
		}
		line += "\n"
		switch {
		case state.TryInitialize(line):
		case state.TrySendPrepare(line):
		case state.TryDeliverPrepareRequest(line):
		case state.TryDeliverPrepareResponse(line):
		case state.TryDeliverAcceptRequest(line):
		case state.TryDeliverAcceptResponse(line):
		case state.TryDeliverDecideRequest(line):
		default:
			log.Fatalf("unknown line: %s", line)
		}
	}
	if err := scanner.Err(); err != nil {
		log.Fatalf("scanner failure: %v", err)
	}
}
