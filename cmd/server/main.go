package main

import (
	"encoding/json"
	"fmt"
)

type Type string
type MyInterface interface {
	Type() Type
}

type StructA struct {
	A float64 `json:"a"`
}
type StructB struct {
	B string `json:"b"`
}
type StructX struct {
	X           string      `json:"x"`
	MyInterface MyInterface `json:"my_interface"`
}

type StructXRAW struct {
	X           string          `json:"x"`
	MyInterface json.RawMessage `json:"my_interface"`
}

func (StructA) Type() Type {
	return "StructA"
}

func (StructB) Type() Type {
	return "StructB"
}

// Check that we have implemented the interface
var _ MyInterface = (*StructA)(nil)
var _ MyInterface = (*StructB)(nil)

func (x StructX) MarshalJSON() ([]byte, error) {
	var xr struct {
		X               string      `json:"x"`
		MyInterface     MyInterface `json:"my_interface"`
		MyInterfaceType Type        `json:"my_interface_type"`
	}
	xr.X = x.X
	xr.MyInterface = x.MyInterface
	xr.MyInterfaceType = x.MyInterface.Type()
	return json.Marshal(xr)
}

func (x *StructX) UnmarshalJSON(b []byte) error {
	var xr struct {
		X               string          `json:"x"`
		MyInterface     json.RawMessage `json:"my_interface"`
		MyInterfaceType Type            `json:"my_interface_type"`
	}
	err := json.Unmarshal(b, &xr)
	if err != nil {
		return err
	}
	x.X = xr.X
	var myInterface MyInterface
	if xr.MyInterfaceType == "StructA" {
		myInterface = &StructA{}
	} else {
		myInterface = &StructB{}
	}
	err = json.Unmarshal(xr.MyInterface, myInterface)
	if err != nil {
		return err
	}
	x.MyInterface = myInterface
	return nil
}

func main() {
	// Create an instance of each a turn to JSON
	xa := StructX{X: "xyz", MyInterface: StructA{A: 1.23}}
	xb := StructX{X: "xyz", MyInterface: StructB{B: "hello"}}

	xaJSON, _ := json.Marshal(xa)
	xbJSON, _ := json.Marshal(xb)
	println(string(xaJSON))
	println(string(xbJSON))

	var newX StructX
	err := json.Unmarshal(xaJSON, &newX)
	if err != nil {
		panic(err)
	}
	fmt.Printf("%+v %T\n", newX.MyInterface, newX.MyInterface)

	var newY StructX
	err = json.Unmarshal(xbJSON, &newY)
	if err != nil {
		panic(err)
	}
	fmt.Printf("%+v %T\n", newY.MyInterface, newY.MyInterface)
}
