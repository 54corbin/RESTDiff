package main

import (
	"bufio"
	"bytes"
	"container/list"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/antlabs/pcurl"
	jd "github.com/josephburnett/jd/lib"
)

type command struct {
	leftCurl  string
	leftParms *list.List
	leftResp  *list.List
	leftDuration  *list.List
	//====================================
	rightCurl  string
	rightParms *list.List
	rightResp  *list.List
	rightDuration *list.List

	//vvvvvvvvvvvvvvvvvvv
	report *list.List
}


var set = flag.Bool("set", false, "指明待比较json中的数组是有序的")

func main() {

	var file = flag.String("f", "", "存储curl命令的文件路径")

	flag.Parse()

	if *file == "" {
		fmt.Println("请用 -f 指定 存放curl 命令的文件")
		os.Exit(-1)
	}
	f, err := os.Open(*file)
	if err != nil {
		fmt.Printf("Failed to open file [%s]:%s\n", *file, err)
		os.Exit(-1)
	}
	defer f.Close()

	//读取配置文件构造命令对象数组
	cmds := constructCmds(f)

	fmt.Println("cmd.size=", cmds.Len())

	for cmd := cmds.Front(); cmd != nil; cmd = cmd.Next() {

		go func(cmdVal *command){
			wg := new(sync.WaitGroup)
			wg.Add(2)
			go doBatchCurl(wg,cmdVal.leftParms,cmdVal.leftCurl,cmdVal.leftResp,cmdVal.leftDuration)
			go doBatchCurl(wg,cmdVal.rightParms,cmdVal.rightCurl,cmdVal.rightResp,cmdVal.rightDuration)
			wg.Wait()
			doBatchCompare(cmdVal.leftResp, cmdVal.rightResp, cmdVal.report)
		}((cmd.Value).(*command))

		break
		// fmt.Println(cmdVal)

	}

		// break
		// leftParams := strings.Split(leftReq, "@")

		// if len(leftParams) < 2 {
		// 	fmt.Print("Wrong cmd fomat at line: ", count)
		// 	break
		// }

		// leftParm := leftParams[0]
		// leftCurl := leftParams[1]

		// // fmt.Printf("p[o]:%s\np[1]:%s", leftParams[0], leftParams[1])

		// paramsPath := fmt.Sprint(filepath.Dir(f.Name()), string(os.PathSeparator), leftParm, ".txt")
		// fmt.Printf("\nf.Name:%s\nparamsPath:%s", f.Name(), paramsPath)

		// parmFile, err := os.Open(paramsPath)
		// if err != nil {
		// 	fmt.Printf("Failed to open file [%s]:%s\n", paramsPath, err)
		// 	continue
		// }
		// defer parmFile.Close()

		// pfBr := bufio.NewReader(parmFile)
		// for parmLine, err := readLine(pfBr); err == nil || err != io.EOF; parmLine, err = readLine(pfBr) {

		// 	reg, _ := regexp.Compile()
		// 	fmt.Println(parmLine)
		// }

		// break

		// //检查是否是合法的curl命令
		// if strings.HasPrefix(leftReq, "curl") {
		// 	rightReq, err := readLine(br)
		// 	//两条curl命令见不能有间隔
		// 	if err != nil || !strings.HasPrefix(rightReq, "curl") {
		// 		fmt.Println(leftReq + " >>缺少对应命令")
		// 		os.Exit(-1)
		// 	}

		// 	//执行curl 命令 获取接口响应
		// 	leftResp := curlReq(leftReq)
		// 	rightResp := curlReq(rightReq)

		// 	count++
		// 	//构造对比结果文件
		// 	outPath := fmt.Sprintf("./report%d.txt", count)

		// 	//比较响应结果
		// 	compare(string(leftResp), string(rightResp), outPath)

		// }


}

//批量比较所有返回结果
func doBatchCompare(left *list.List,right *list.List,report *list.List){

	if left.Len() != right.Len(){
		fmt.Println("左右结果集大小不匹配")
		report.PushBack("左右结果集大小不匹配")
		return
	}

	rightEle := right.Front()
	for leftEle := left.Front();leftEle != nil;leftEle = leftEle.Next(){
		diff := compare(leftEle.Value.(string), rightEle.Value.(string))
		report.PushBack(diff)
		rightEle = rightEle.Next()
	}

	fmt.Println(*report)

}


//读取文件构造请求/比对 对象数组
func constructCmds(file *os.File) *list.List {

	cmds := list.New()

	br := bufio.NewReader(file)

	for count := 1; ; count++ {

		left, err := readLine(br)
		if err != nil && err == io.EOF {
			fmt.Println("end of file....done")
			break
		}

		if !strings.Contains(left, "curl") || !strings.Contains(left, "@") {
			continue
		}
		right, err := readLine(br)
		if (err != nil && err == io.EOF) || !strings.Contains(right, "curl") || !strings.Contains(right, "@") {
			fmt.Println(left, " >>缺少格式正确的对应命令 行号：", count)
			os.Exit(-1)
		}

		tmpL := strings.Split(left, "@")
		tmpR := strings.Split(left, "@")

		cmd := new(command)
		cmd.leftCurl = tmpL[1]
		cmd.leftCurl = tmpR[1]
		paramsFilePathL := fmt.Sprint(filepath.Dir(file.Name()), string(os.PathSeparator), tmpL[0], ".txt")
		paramsFilePathR := fmt.Sprint(filepath.Dir(file.Name()), string(os.PathSeparator), tmpR[0], ".txt")
		readParms(paramsFilePathL, cmd.leftParms)
		readParms(paramsFilePathR, cmd.leftParms)
		cmds.PushBack(cmd)
	}
	return cmds
}

//用每个不同的参数依次发送请求
func doBatchCurl(wg *sync.WaitGroup, parms *list.List,curl string,respList *list.List,durationList *list.List) {
	defer wg.Done()
	for parmLine := parms.Front(); parmLine != nil && strings.Contains(parmLine.Value.(string), "@@"); parmLine = parmLine.Next() {

		parms := strings.Split((parmLine.Value).(string), "@@")

		for i, parm := range parms {

			curl = strings.Replace(curl, "$$$", parm, i+1)

		}

		leftResp, ld := curlReq(curl)
		respList.PushBack(leftResp)
		durationList.PushBack(ld)
	}
}

//将参数文件按行存入list
func readParms(filePath string, li *list.List) {

	f, err := os.Open(filePath)
	if err != nil {
		fmt.Printf("Failed to open file [%s]:%s\n", filePath, err)
		os.Exit(-1)
	}
	defer f.Close()

	pfBr := bufio.NewReader(f)
	for line, err := readLine(pfBr); err == nil || err != io.EOF; line, err = readLine(pfBr) {

		li.PushBack(line)
	}

}

//do curl http request
func curlReq(cmd string) ([]byte,time.Duration) {

	start := time.Now()
	req, err := pcurl.ParseAndRequest(cmd)
	if err != nil {
		fmt.Printf("err:%s\n", err)
		return nil,-1
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("err:%s\n", err)
		return nil,-1
	}

	duration := time.Since(start)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)

	fmt.Printf("\n%s\nVVVVVVVVVVVVVVVVVVVVVV\n%s\n\n", cmd, string(body))
	return body,duration
}

//从reader里读取一行
func readLine(reader *bufio.Reader) (string, error) {
	line, err := reader.ReadBytes('\n')
	line = bytes.TrimRight(line, "\r\n")
	return string(line), err

}

/**
 *比较两个json字符串并将结果存入 outPath指定的文件
**/
func compare(lj string, rj string) string {

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

	return diff

	// ioutil.WriteFile(outPath, []byte(diff), 0644)
	// fmt.Println("写入对比结果:", outPath)
}
