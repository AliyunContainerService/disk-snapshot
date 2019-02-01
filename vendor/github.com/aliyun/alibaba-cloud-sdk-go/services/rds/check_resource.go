package rds

//Licensed under the Apache License, Version 2.0 (the "License");
//you may not use this file except in compliance with the License.
//You may obtain a copy of the License at
//
//http://www.apache.org/licenses/LICENSE-2.0
//
//Unless required by applicable law or agreed to in writing, software
//distributed under the License is distributed on an "AS IS" BASIS,
//WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//See the License for the specific language governing permissions and
//limitations under the License.
//
// Code generated by Alibaba Cloud SDK Code Generator.
// Changes may cause incorrect behavior and will be lost if the code is regenerated.

import (
	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/requests"
	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/responses"
)

// CheckResource invokes the rds.CheckResource API synchronously
// api document: https://help.aliyun.com/api/rds/checkresource.html
func (client *Client) CheckResource(request *CheckResourceRequest) (response *CheckResourceResponse, err error) {
	response = CreateCheckResourceResponse()
	err = client.DoAction(request, response)
	return
}

// CheckResourceWithChan invokes the rds.CheckResource API asynchronously
// api document: https://help.aliyun.com/api/rds/checkresource.html
// asynchronous document: https://help.aliyun.com/document_detail/66220.html
func (client *Client) CheckResourceWithChan(request *CheckResourceRequest) (<-chan *CheckResourceResponse, <-chan error) {
	responseChan := make(chan *CheckResourceResponse, 1)
	errChan := make(chan error, 1)
	err := client.AddAsyncTask(func() {
		defer close(responseChan)
		defer close(errChan)
		response, err := client.CheckResource(request)
		if err != nil {
			errChan <- err
		} else {
			responseChan <- response
		}
	})
	if err != nil {
		errChan <- err
		close(responseChan)
		close(errChan)
	}
	return responseChan, errChan
}

// CheckResourceWithCallback invokes the rds.CheckResource API asynchronously
// api document: https://help.aliyun.com/api/rds/checkresource.html
// asynchronous document: https://help.aliyun.com/document_detail/66220.html
func (client *Client) CheckResourceWithCallback(request *CheckResourceRequest, callback func(response *CheckResourceResponse, err error)) <-chan int {
	result := make(chan int, 1)
	err := client.AddAsyncTask(func() {
		var response *CheckResourceResponse
		var err error
		defer close(result)
		response, err = client.CheckResource(request)
		callback(response, err)
		result <- 1
	})
	if err != nil {
		defer close(result)
		callback(nil, err)
		result <- 0
	}
	return result
}

// CheckResourceRequest is the request struct for api CheckResource
type CheckResourceRequest struct {
	*requests.RpcRequest
	ResourceOwnerId      requests.Integer `position:"Query" name:"ResourceOwnerId"`
	ResourceOwnerAccount string           `position:"Query" name:"ResourceOwnerAccount"`
	OwnerAccount         string           `position:"Query" name:"OwnerAccount"`
	SpecifyCount         string           `position:"Query" name:"SpecifyCount"`
	EngineVersion        string           `position:"Query" name:"EngineVersion"`
	OwnerId              requests.Integer `position:"Query" name:"OwnerId"`
	DBInstanceClass      string           `position:"Query" name:"DBInstanceClass"`
	Engine               string           `position:"Query" name:"Engine"`
	ZoneId               string           `position:"Query" name:"ZoneId"`
	DBInstanceUseType    string           `position:"Query" name:"DBInstanceUseType"`
	DBInstanceId         string           `position:"Query" name:"DBInstanceId"`
}

// CheckResourceResponse is the response struct for api CheckResource
type CheckResourceResponse struct {
	*responses.BaseResponse
	RequestId    string    `json:"RequestId" xml:"RequestId"`
	SpecifyCount string    `json:"SpecifyCount" xml:"SpecifyCount"`
	Resources    Resources `json:"Resources" xml:"Resources"`
}

// CreateCheckResourceRequest creates a request to invoke CheckResource API
func CreateCheckResourceRequest() (request *CheckResourceRequest) {
	request = &CheckResourceRequest{
		RpcRequest: &requests.RpcRequest{},
	}
	request.InitWithApiInfo("Rds", "2014-08-15", "CheckResource", "rds", "openAPI")
	return
}

// CreateCheckResourceResponse creates a response to parse from CheckResource response
func CreateCheckResourceResponse() (response *CheckResourceResponse) {
	response = &CheckResourceResponse{
		BaseResponse: &responses.BaseResponse{},
	}
	return
}