/*
Copyright IBM Corp 2016 All Rights Reserved.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
		 http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"errors"
	"fmt"
    "encoding/json"
	"github.com/hyperledger/fabric/core/chaincode/shim"
	"github.com/joerust/referral-partners/partnerlogic"
)

type PartnerChaincode struct {
	PartnerName string
}

type PaycorReferral struct {
	ReferralId string `json:"referralId"`
    CustomerName string `json:"customerName"`
	ContactNumber int64 `json:"contactNumber"`
	CreateDate int64 `json:"createDate"`
	Status string `json:"status"`
	CustomerSize *string `json:"customerSize"`
	Compensation *string `json:"compensation"`
	PartnerName *string `json:"partnerName"`
	DealCriteria *string `json:"dealCriteria"`
}

func main() {
	err := shim.Start(new(PartnerChaincode))
	if err != nil {
		fmt.Printf("Error starting Simple chaincode: %s", err)
	}	
}

// Init resets all the things
func (t *PartnerChaincode) Init(stub *shim.ChaincodeStub, function string, args []string) ([]byte, error) {
	// Initialize the partner names
	t.PartnerName = args[0]
	fmt.Println("Initializing chaincode for partner: " + t.PartnerName)
	return nil, nil
}

// Invoke is our entry point to invoke a chaincode function
func (t *PartnerChaincode) Invoke(stub *shim.ChaincodeStub, function string, args []string) ([]byte, error) {
	fmt.Println("invoke is running " + function)

	// Handle different functions
	if function == "init" {
		return t.Init(stub, "init", args)
	} else if function == "createReferral" {
		return t.createReferral(stub, args)
	} else if function == "updateReferralStatus" {
		return t.updateReferralStatus(stub, args)
	} 
	fmt.Println("invoke did not find func: " + function)

	return nil, errors.New("Received unknown function invocation")
}

// Query is our entry point for queries
func (t *PartnerChaincode) Query(stub *shim.ChaincodeStub, function string, args []string) ([]byte, error) {
	fmt.Println("query is running " + function)

	// Handle different functions
	if function == "read" { //read a variable
		return t.read(stub, args)
	} else if function == "searchByStatus" {
		return t.searchByStatus(args[0], stub)
	}
	
	fmt.Println("query did not find func: " + function)

	return nil, errors.New("Received unknown function query")
}

// updateReferral - invoke function to updateReferral key/value pair
func (t *PartnerChaincode) updateReferralStatus(stub *shim.ChaincodeStub, args []string) ([]byte, error) {
	var key, value string
	var err error
	var referral PaycorReferral
	var valAsbytes []byte
	
	fmt.Println("running updateReferral()")

	if len(args) != 2 {
		return nil, errors.New("Incorrect number of arguments. Expecting 2. name of the key and value to set")
	}

	key = args[0] // The referral id
	value = args[1] // The new status
	
	// Look up the json blob that matches the current referral id
	valAsbytes, err = stub.GetState(key)
	
	// Unmarshall said json blob into a referral object
	err = json.Unmarshal(valAsbytes, &referral)
	
	// Save the current status so that it can be unindexed once we update the referral object
	oldStatus := referral.Status;
	
	// Set the referral status to the new value
	referral.Status = value;
	
	// Serialize the object to a JSON string to be stored in the ledger
	valAsbytes, err = json.Marshal(referral)
	
	// Store the json string in the ledger
	err = stub.PutState(key, valAsbytes) //write the variable into the chaincode state
	
	if err != nil {
		return nil, err
	}
	
	// Index things by the new status
	err = partnerlogic.IndexByStatus(key, referral.Status, stub)
	
	if err != nil {
		return []byte("Count not index the bytes by status from the value: " + value + " on the ledger"), err
	}
	
	// Remove the indexing by the status before the update
	err = partnerlogic.RemoveStatusReferralIndex(key, oldStatus, stub)
	
	return valAsbytes, nil
}

// createReferral - invoke function to write key/value pair
func (t *PartnerChaincode) createReferral(stub *shim.ChaincodeStub, args []string) ([]byte, error) {

	var referralKey, referralData string
	var err error
	fmt.Println("running createReferral()")

	if len(args) != 2 {
		return nil, errors.New("Incorrect number of arguments. Expecting 2 parameters, name of the key and value to set")
	}

	referralKey = args[0] //rename for funsies
	referralData = args[1]
	
	err = stub.PutState(referralKey, []byte(referralData)) //write the variable into the chaincode state
	if err != nil {
		return nil, err
	}
	
	var referral PaycorReferral

	err = json.Unmarshal([] byte(referralData), &referral)

	
	// Deserialize the input string into a GO data structure to hold the referral
	err = partnerlogic.IndexByStatus(referralKey, referral.Status, stub)
	
	if err != nil {
		return []byte("Count not index the bytes by status from the value: " + referralData + " on the ledger"), err
	}
	
	return [] byte(referralData), nil
}

func (t *PartnerChaincode) searchByStatus(status string, stub *shim.ChaincodeStub) ([]byte, error) {
	valAsbytes, err := stub.GetState(status)
	
	if err != nil {
		jsonResp := "{\"Error\":\"Failed to get state for " + status + "\"}"
		return nil, errors.New(jsonResp)
	}
	
	valAsbytes, err = partnerlogic.ProcessCommaDelimitedReferrals(partnerlogic.BytesToString(valAsbytes), stub)
	
	if(err != nil) {
		return nil, err
	}
	
	return valAsbytes, nil
}


// read - query function to read key/value pair
func (t *PartnerChaincode) read(stub *shim.ChaincodeStub, args []string) ([]byte, error) {
	var key, jsonResp string
	var err error
	
	if len(args) != 1 {
		return nil, errors.New("Incorrect number of arguments. Expecting name of the key to query")
	}

	key = args[0]
	valAsbytes, err := stub.GetState(key)
	
	if err != nil {
		jsonResp = "{\"Error\":\"Failed to get state for " + key + "\"}"
		return []byte(jsonResp), err
	}
	
	if valAsbytes == nil {
		return []byte("Did not find entry for key: " + key), nil
	}
	return valAsbytes, nil
}