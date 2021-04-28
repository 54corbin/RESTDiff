package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/antlabs/pcurl"
	"github.com/yazgazan/jaydiff/diff"
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

func main() {

	f, err := os.Open("requests.txt")
	if err != nil {
		fmt.Printf("open file failed:%s", err)
	}
	defer f.Close()

	br := bufio.NewReader(f)
	for {
		line, err := br.ReadBytes('\n')
		line = bytes.TrimRight(line, "\r\n")
		if err != nil && err == io.EOF {
			break
		}
		lineStr := string(line)
		// fmt.Println(lineStr)

		if strings.HasPrefix(lineStr, "curl") {
			resp := curlReq(lineStr)
			// fmt.Println(resp)

			var lhs interface{}
			var rhs interface{}

			fmt.Println("unmarshal response.......")
			if err := json.Unmarshal(resp, &lhs); nil != err {

				fmt.Printf("unmarshal failed:%s", err)
				if e, ok := err.(*json.SyntaxError); ok {
					fmt.Printf("\nsyntax error at %d", e.Offset)

					for i, c := range resp {
						fmt.Println(i, c)

					}
				}
			}

			json.Unmarshal(resp, &rhs)

			fmt.Println(lhs)
			fmt.Println(rhs)

			differ, err := diff.Diff(lhs,rhs,nil)
			if nil != err{

				fmt.Println(err)
			}
			fmt.Println(differ)
		}
	}

}
