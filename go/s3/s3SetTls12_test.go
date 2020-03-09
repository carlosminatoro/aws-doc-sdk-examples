/*
   Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.

   This file is licensed under the Apache License, Version 2.0 (the "License").
   You may not use this file except in compliance with the License. A copy of
   the License is located at

    http://aws.amazon.com/apache2.0/

   This file is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR
   CONDITIONS OF ANY KIND, either express or implied. See the License for the
   specific language governing permissions and limitations under the License.
*/

package main

import (
    "crypto/tls"
    "encoding/json"
    "fmt"
    "io/ioutil"
    "net/http"
    "strings"
    "testing"

    "github.com/google/uuid"

    "github.com/aws/aws-sdk-go/aws"
    "github.com/aws/aws-sdk-go/aws/session"
    "github.com/aws/aws-sdk-go/service/s3"
    "github.com/aws/aws-sdk-go/service/s3/s3manager"

    "golang.org/x/net/http2"
)

type Config struct {
    Region    string  `json:"Region"`
    GoVersion float32 `json:"GoVersion"`
}

var configFileName = "config.json"

var globalConfig Config

func populateConfiguration(t *testing.T) error {
    content, err := ioutil.ReadFile(configFileName)
    if err != nil {
        return err
    }

    text := string(content)

    err = json.Unmarshal([]byte(text), &globalConfig)
    if err != nil {
        return err
    }

    t.Log("Region:      " + globalConfig.Region)
    t.Log("GoVersion:   " + fmt.Sprintf("%f", globalConfig.GoVersion))

    return nil
}

func createBucketAndItem(sess *session.Session, bucketName *string, itemName *string) error {
    svc := s3.New(sess)
    _, err := svc.CreateBucket(&s3.CreateBucketInput{
        Bucket: bucketName,
    })
    if err != nil {
        return err
    }

    err = svc.WaitUntilBucketExists(&s3.HeadBucketInput{
        Bucket: bucketName,
    })
    if err != nil {
        return err
    }

    _, err = svc.PutObject(&s3.PutObjectInput{
        Body:   strings.NewReader("Hello World!"),
        Bucket: bucketName,
        Key:    itemName,
    })

    return err
}

func deleteBucket(sess *session.Session, bucketName *string) error {
    svc := s3.New(sess)

    iter := s3manager.NewDeleteListIterator(svc, &s3.ListObjectsInput{
        Bucket: bucketName,
    })

    err := s3manager.NewBatchDeleteWithClient(svc).Delete(aws.BackgroundContext(), iter)
    if err != nil {
        return err
    }

    _, err = svc.DeleteBucket(&s3.DeleteBucketInput{
        Bucket: bucketName,
    })
    if err != nil {
        return err
    }

    return svc.WaitUntilBucketNotExists(&s3.HeadBucketInput{
        Bucket: bucketName,
    })
}

func TestTLSVersion(t *testing.T) {
    err := populateConfiguration(t)
    if err != nil {
        t.Fatal("Could not get configuration values")
    }

    // Create HTTP client with minimum TLS version
    tr := &http.Transport{
        TLSClientConfig: &tls.Config{
            MinVersion: tls.VersionTLS12,
        },
    }

    minGo := (float32)(1.12)

    if globalConfig.GoVersion > minGo {
        tr.ForceAttemptHTTP2 = true
        t.Log("Created TLS 1.2 for Go version 1.13")
    } else {
        err := http2.ConfigureTransport(tr)
        t.Log("Created TLS 1.12 for Go version 1.12 (or previous)")
        if err != nil {
            t.Fatal(err)
        }
    }

    // Create an HTTP client with the configured transport.
    client := http.Client{Transport: tr}

    // Create the SDK's session with the custom HTTP client.
    sess, err := session.NewSession(&aws.Config{
        HTTPClient: &client,
    })
    if err != nil {
        t.Fatal(err)
    }

    version := GetTLSVersion(tr)

    t.Log("Your TLS version: " + version)

    // Set region, bucket, item values
    defaultRegion := "us-west-2"
    if globalConfig.Region == "" {
        t.Log("Setting region to " + defaultRegion)
        globalConfig.Region = defaultRegion
    }

    item := "testitem"

    id := uuid.New()
    bucket := "testbucket-" + id.String()

    // Create the bucket and item
    err = createBucketAndItem(sess, &bucket, &item)
    if err != nil {
        t.Fatal(err)
    }

    t.Log("Created bucket: " + bucket)
    t.Log("With item :     " + item)

    err = ConfirmBucketItemExists(sess, &bucket, &item)
    if err != nil {
        t.Fatal(err)
    }

    t.Log("Bucket " + bucket + " and item " + item + " can be accessed")

    err = deleteBucket(sess, &bucket)
    if err != nil {
        t.Log("You will have to delete bucket " + bucket)
        t.Fatal(err)
    }

}
