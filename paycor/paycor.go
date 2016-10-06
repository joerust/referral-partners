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
	"reflect"
	"unsafe"
	"strings"
	"strconv"
	"github.com/hyperledger/fabric/core/chaincode/shim"
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
	BranchId string `json:"branchId"`
	CustomerSize string `json:"customerSize"`
	Compensation *int64 `json:"compensation"`
	PartnerName string `json:"partnerName"`
	DealCriteria string `json:"dealCriteria"`
}

func main() {
	err := shim.Start(new(PartnerChaincode))
	if err != nil {
		fmt.Printf("Error starting Simple chaincode: %s", err)
	}	
}

func BytesToString(b []byte) string {
    if b == nil {
	    return ""
	}
	
    bh := (*reflect.SliceHeader)(unsafe.Pointer(&b))
    sh := reflect.StringHeader{bh.Data, bh.Len}
    return *(*string)(unsafe.Pointer(&sh))
}

func RemoveStatusReferralIndex(referralId string, status string, stub *shim.ChaincodeStub) (error) {
	valAsbytes, err := stub.GetState(status)
	if err != nil {
		jsonResp := "{\"Error\":\"Failed to get state for " + status + "\"}"
		return errors.New(jsonResp)
	}
	
	if valAsbytes == nil {
		return nil;
	} else {
		// Remove the referral from this status type, if it exists
		commaDelimitedStatuses := BytesToString(valAsbytes)
		referralIdsInCurrentStatus := strings.Split(commaDelimitedStatuses, ",")
		updatedReferralIdList := ""
		
		appendComma := false
		for i := range referralIdsInCurrentStatus {
			if referralIdsInCurrentStatus[i] != referralId {
			    if appendComma == false {
					updatedReferralIdList += referralIdsInCurrentStatus[i]
					appendComma = true
				} else {
					updatedReferralIdList = updatedReferralIdList + "," + referralIdsInCurrentStatus[i]
				}
			}
		}
		
		err = stub.PutState(status, []byte(updatedReferralIdList))
	}
	
	if err != nil {
		jsonResp := "{\"Error\":\"Failed to update state for " + status + "\"}"
		return errors.New(jsonResp)
	}
	
	return nil
}

// Adds the referral id to a ledger list item for the given department allowing for quick search of referrals in a given department
func IndexByStatus(referralId string, status string, stub *shim.ChaincodeStub) (error) {
	valAsbytes, err := stub.GetState(status)
	if err != nil {
		jsonResp := "{\"Error\":\"Failed to get state for " + status + "\"}"
		return errors.New(jsonResp)
	}
	
	if valAsbytes == nil {
		err = stub.PutState(status, []byte(referralId))
	} else {
	    commaDelimitedStatuses := BytesToString(valAsbytes)
		err = stub.PutState(status, []byte(commaDelimitedStatuses + "," + referralId))
	}
	
	if err != nil {
		jsonResp := "{\"Error\":\"Failed to update state for " + status + "\"}"
		return errors.New(jsonResp)
	}
	
	return nil
}

func ProcessCommaDelimitedReferrals(delimitedReferrals string, stub *shim.ChaincodeStub) ([]byte, error) {
	commaDelimitedReferrals := strings.Split(delimitedReferrals, ",")

	referralResultSet := "["
	
	for i := range commaDelimitedReferrals {
		valAsbytes, err := stub.GetState(commaDelimitedReferrals[i])
		
		if err != nil {
			return nil, err
		}
		
		if i == 0 {
			referralResultSet = referralResultSet + BytesToString(valAsbytes)
		} else {
			referralResultSet = referralResultSet + "," + BytesToString(valAsbytes)
		}
	}
	
	referralResultSet += "]"
	return []byte(referralResultSet), nil
}

// Init resets all the things
func (t *PartnerChaincode) Init(stub *shim.ChaincodeStub, function string, args []string) ([]byte, error) {
	// Initialize the partner names
	t.PartnerName = "Paycor"
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
	} else if function == "closeReferredDeal" {
		return t.closeReferredDeal(stub, args)
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
	} else if function == "readAllReferrals" {
		return t.readAllReferrals(stub)
	}
	
	fmt.Println("query did not find func: " + function)

	return nil, errors.New("Received unknown function query")
}

func (t *PartnerChaincode) closeReferredDeal(stub *shim.ChaincodeStub, args []string) ([]byte, error) {
	var referralId, dealCriteria string
	var err error
	var referral PaycorReferral
	var referralAsBytes []byte
	var closingCommission [3][4]int64
	var companySizeIndex,dealSizeIndex int
	
	closingCommission[0] = [...]int64{250,300,350,400}
	closingCommission[1] = [...]int64{1000,1250,1500,1750}
	closingCommission[2] = [...]int64{2000,2500,3000,3500}
	
	fmt.Println("running closeReferredDeal()")

	if len(args) != 2 {
		return nil, errors.New("Incorrect number of arguments. Expecting 2. name of the key and value to set")
	}

	referralId = args[0] // The referral id
	dealCriteria = args[1] // The new deal criteria
	
	// Look up the json blob that matches the current referral id
	referralAsBytes, err = stub.GetState(referralId)
	
	// Unmarshall said json blob into a referral object
	err = json.Unmarshal(referralAsBytes, &referral)
	
	// Save the current status so that it can be unindexed once we update the referral object
	oldStatus := referral.Status
	
	// Set the referral status to the new value
	referral.Status = "CLOSED"
	referral.DealCriteria = dealCriteria
	
	if dealCriteria == "SMALL" {
	   dealSizeIndex = 0
	} else if dealCriteria == "MID" {
		dealSizeIndex = 1
	} else {
		dealSizeIndex = 2
	}
	
	if referral.CustomerSize == "MICRO" {
		companySizeIndex = 0
	} else if referral.CustomerSize == "SMALL" {
		companySizeIndex = 1
	} else if referral.CustomerSize == "MID" {
		companySizeIndex = 2
	} else {
		companySizeIndex = 3
	}
	
	fmt.Println("Paying out a commission of: " + strconv.FormatInt(closingCommission[dealSizeIndex][companySizeIndex], 10))
	
	referral.Compensation = &(closingCommission[dealSizeIndex][companySizeIndex])
	
	
	// Serialize the object to a JSON string to be stored in the ledger
	referralAsBytes, err = json.Marshal(referral)
	
	// Store the json string in the ledger
	err = stub.PutState(referralId, referralAsBytes) //write the variable into the chaincode state
	
	if err != nil {
		return nil, err
	}
	
	// Index things by the new status
	err = IndexByStatus(referralId, referral.Status, stub)
	
	if err != nil {
		return []byte("Count not index the bytes by status from the value: " + referral.Status + " on the ledger"), err
	}
	
	// Remove the indexing by the status before the update
	err = RemoveStatusReferralIndex(referralId, oldStatus, stub)
	
	return referralAsBytes, nil
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
	err = IndexByStatus(key, referral.Status, stub)
	
	if err != nil {
		return []byte("Count not index the bytes by status from the value: " + value + " on the ledger"), err
	}
	
	// Remove the indexing by the status before the update
	err = RemoveStatusReferralIndex(key, oldStatus, stub)
	
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
	err = IndexByStatus(referralKey, referral.Status, stub)
	
	if err != nil {
		return []byte("Count not index the bytes by status from the value: " + referralData + " on the ledger"), err
	}
	
	return [] byte(referralData), nil
}

func (t *PartnerChaincode) readAllReferrals(stub *shim.ChaincodeStub) ([]byte, error) {
	var err error
	var activeStatusesAsBytes []byte
	var declinedStatusesAsBytes []byte
	var pendingStatusesAsBytes []byte
	var closedStatusesAsBytes []byte
	var allReferralsAsbytes []byte
	var activeReferrals []PaycorReferral
	var declinedReferrals []PaycorReferral
	var pendingReferrals []PaycorReferral
	var closedReferrals []PaycorReferral
	var allReferrals []PaycorReferral
	
	fmt.Println("Reading active referrals")
	activeStatusesAsBytes, err = t.searchByStatus("ACTIVE", stub)
	if err != nil {
		jsonResp := "{\"Error\":\"Failed to get state for ACTIVE\"}"
		return []byte(err.Error()), errors.New(jsonResp)
	}
	
	fmt.Println("Unmarshalling active referrals")
	err = json.Unmarshal(activeStatusesAsBytes, &activeReferrals)
	if err != nil {
		jsonResp := "{\"Error\":\"Failed to get state for ACTIVE\"}"
		return []byte(err.Error()), errors.New(jsonResp)
	}
	
	fmt.Println("Reading declined referrals")
	declinedStatusesAsBytes, err = t.searchByStatus("DECLINED", stub)
	if err != nil {
		jsonResp := "{\"Error\":\"Failed to get state for DECLINED\"}"
		return []byte(err.Error()), errors.New(jsonResp)
	}
	
	fmt.Println("Unmarshalling declined referrals")
	err = json.Unmarshal(declinedStatusesAsBytes, &declinedReferrals)
	if err != nil {
		jsonResp := "{\"Error\":\"Failed to get state for DECLINED\"}"
		return []byte(err.Error()), errors.New(jsonResp)
	}
	
	fmt.Println("Reading pending referrals")
	pendingStatusesAsBytes, err = t.searchByStatus("PENDING", stub)
	if err != nil {
		jsonResp := "{\"Error\":\"Failed to get state for PENDING\"}"
		return []byte(err.Error()), errors.New(jsonResp)
	}
	
	fmt.Println("Unmarshalling pending referrals")
	err = json.Unmarshal(pendingStatusesAsBytes, &pendingReferrals)
	if err != nil {
		jsonResp := "{\"Error\":\"Failed to get state for PENDING\"}"
		return []byte(err.Error()), errors.New(jsonResp)
	}
	
	fmt.Println("Reading closed referrals")
	closedStatusesAsBytes, err = t.searchByStatus("CLOSED", stub)
	if err != nil {
		jsonResp := "{\"Error\":\"Failed to get state for CLOSED\"}"
		return []byte(err.Error()), errors.New(jsonResp)
	}
	
	fmt.Println("Unmarshalling closed referrals")
	err = json.Unmarshal(closedStatusesAsBytes, &closedReferrals)
	if err != nil {
		jsonResp := "{\"Error\":\"Failed to get state for CLOSED\"}"
		return []byte(err.Error()), errors.New(jsonResp)
	}
	
	if(err != nil) {
		return []byte(err.Error()), err
	}
	
	fmt.Println("Appending active referrals")
	allReferrals = append(allReferrals, activeReferrals...)
	
	fmt.Println("Appending declined referrals")
	allReferrals = append(allReferrals, declinedReferrals...)
	
	fmt.Println("Appending pending referrals")
	allReferrals = append(allReferrals, pendingReferrals...)
	
	fmt.Println("Appending closed referrals")
	allReferrals = append(allReferrals, closedReferrals...)
	
	fmt.Println("Marshalling all referrals")
	allReferralsAsbytes, err = json.Marshal(allReferrals)
	if(err != nil) {
		fmt.Println(err.Error())
		return []byte(err.Error()), err
	}
	
	fmt.Println("Returing all referrals: " + BytesToString(allReferralsAsbytes))
	return allReferralsAsbytes, nil
}

func (t *PartnerChaincode) searchByStatus(status string, stub *shim.ChaincodeStub) ([]byte, error) {
	valAsbytes, err := stub.GetState(status)
	
	if err != nil {
		jsonResp := "{\"Error\":\"Failed to get state for " + status + "\"}"
		return nil, errors.New(jsonResp)
	}
	
	valAsbytes, err = ProcessCommaDelimitedReferrals(BytesToString(valAsbytes), stub)
	
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