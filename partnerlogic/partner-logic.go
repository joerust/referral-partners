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

package partnerlogic

import (
	"errors"
	"reflect"
	"unsafe"
	"strings"
	"github.com/hyperledger/fabric/core/chaincode/shim"
)

func BytesToString(b []byte) string {
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

func FindAllReferrals(stub *shim.ChaincodeStub, partnerName string) ([]byte, error) {
	valAsbytes, err := stub.GetState(partnerName)
	
	if err != nil {
		jsonResp := "{\"Error\":\"Failed to get state for " + partnerName + "\"}"
		return nil, errors.New(jsonResp)
	}
	
	valAsbytes, err = ProcessCommaDelimitedReferrals(BytesToString(valAsbytes), stub)
	
	if(err != nil) {
		return nil, err
	}
	
	return valAsbytes, nil
}

func SearchByStatus(status string, stub *shim.ChaincodeStub) ([]byte, error) {
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
func Read(stub *shim.ChaincodeStub, args []string) ([]byte, error) {
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