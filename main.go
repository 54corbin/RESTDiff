package main

import (
	"bufio"
	"bytes"
	"flag"
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

	fmt.Printf("\n%s\nVVVVVVVVVVVVVVVVVVVVVV\n%s\n\n", cmd, string(body))
	return body
}

/**
 *从reader里读取一行
**/
func readLine(reader *bufio.Reader) (string, error) {
	line, err := reader.ReadBytes('\n')
	line = bytes.TrimRight(line, "\r\n")
	return string(line), err

}

/**
 *比较两个json字符串并将结果存入 outPath指定的文件
**/
func compare(lj string, rj string, outPath string) {

	a, err := jd.ReadJsonString(lj)
	if nil != err {
		fmt.Printf("error：%s\n", err)
	}
	b, err2 := jd.ReadJsonString(rj)
	if nil != err2 {
		fmt.Printf("error：%s\n", err2)
	}


	var diff string
	if *set {
		//添加 -set 参数指明待比较json中的数组是无序的
		metadata := make([]jd.Metadata, 0)
		metadata = append(metadata, jd.SET)
		diff = a.Diff(b, metadata...).Render()
	} else {
		diff = a.Diff(b).Render()
	}

	ioutil.WriteFile(outPath, []byte(diff), 0644)
	fmt.Println("写入对比结果:", outPath)
}


var set = flag.Bool("set", false, "指明待比较json中的数组是有序的")
var file = flag.String("f", "", "存储curl命令的文件路径")

func main() {


	flag.Parse()

	if *file == "" {
		fmt.Println("请用 -f 指定 存放curl 命令的文件")
		os.Exit(-1)
	}

	f, err := os.Open(*file)
	if err != nil {
		fmt.Printf("Failed to open file [%s]:%s",*file, err)
		os.Exit(-1)
	}
	defer f.Close()

	br := bufio.NewReader(f)

	var count int32 = 0
	for {
		leftReq, err := readLine(br)
		if err != nil && err == io.EOF {
			fmt.Println("end of file....done")
			break
		}
		// fmt.Println()

		//检查是否是合法的curl命令
		if strings.HasPrefix(leftReq, "curl") {
			rightReq, err := readLine(br)
			//两条curl命令见不能有间隔
			if err != nil || !strings.HasPrefix(rightReq, "curl") {
				fmt.Println(leftReq + " >>缺少对应命令")
				os.Exit(-1)
			}

			//执行curl 命令 获取接口响应
			leftResp := curlReq(leftReq)
			rightResp := curlReq(rightReq)

			count++
			//构造对比结果文件
			outPath := fmt.Sprintf("./report%d.txt", count)

			//比较响应结果
			compare(string(leftResp), string(rightResp), outPath)

		}
	}

}
