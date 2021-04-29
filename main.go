package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/antlabs/pcurl"
	jd "github.com/josephburnett/jd/lib"
)

func curlReq(cmd string) []byte {

	req, err := pcurl.ParseAndRequest(cmd)
	if err != nil {
		fmt.Printf("err:%s\n", err)
		return nil
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("err:%s\n", err)
		return nil
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	return body
}

/**
  *从reader里读取一行
 **/
func readLine( reader *bufio.Reader)(string, error){
	line, err := reader.ReadBytes('\n')
	line = bytes.TrimRight(line, "\r\n")
	return string(line),err

}

func compare(lj string, rj string, outPath string){

	a,err := jd.ReadJsonString(lj)
	if nil != err {
		fmt.Printf("error：%s\n",err)
	}
	b,err2 := jd.ReadJsonString(rj)
	ioutil.WriteFile(report, []byte(diff), 0644)
}

func main() {

	f, err := os.Open("requests.txt")
	if err != nil {
		fmt.Printf("open file failed:%s", err)
	}
	defer f.Close()

	br := bufio.NewReader(f)

	var count int32
	for {
		leftReq, err := readLine(br)
		if err != nil && err == io.EOF {
			fmt.Println("end of file....done")
			break
		}
		// fmt.Println()

		if strings.HasPrefix(leftReq, "curl") {
			rightReq, _ := readLine(br)
			//两条curl命令见不能有间隔
			if !strings.HasPrefix(rightReq, "curl") {
				fmt.Println("文件格式错误 at:"+leftReq)
			}

			leftResp := curlReq(leftReq)
			rightResp := curlReq(rightReq)

			outPath := "./reort"+string(count)+".txt"

			compare(string(leftResp),string(rightResp),outPath)

		}
	}

}
