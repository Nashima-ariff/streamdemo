package main

import (
	"awesomeProject/header"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"io"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"os"
	"regexp"
	"strconv"
)

type Range struct {
	Start          int64
	End            int64
	Length         int64
	TotalMediaSize int64
}
type ResponseStatus int32

const (
	SUCCESS ResponseStatus = 0
	FAILED                 = 1
)

type GetContentByChunkResponse struct {
	Status           ResponseStatus `protobuf:"varint,1,opt,name=status,proto3,enum=ResponseStatus" json:"status,omitempty"`
	ContentId        int64          `protobuf:"varint,2,opt,name=contentId,proto3" json:"contentId,omitempty"`
	ChunkId          int64          `protobuf:"varint,3,opt,name=chunkId,proto3" json:"chunkId,omitempty"`
	ChunkSize        int64          `protobuf:"varint,4,opt,name=chunkSize,proto3" json:"chunkSize,omitempty"`
	TotalContentSize int64          `protobuf:"varint,5,opt,name=totalContentSize,proto3" json:"totalContentSize,omitempty"`
	ContentData      []byte         `protobuf:"bytes,6,opt,name=contentData,proto3" json:"contentData,omitempty"`
	Message          string         `protobuf:"bytes,7,opt,name=message,proto3" json:"message,omitempty"`
}

var rangeExpression = regexp.MustCompile(`^bytes=(\d+)-(\d*)`)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	//r := gin.Default()
	r := gin.New()
	getMediaInfo()
	//createChunk()
	r.GET("/server/:id", func(c *gin.Context) {
		data := duplicateServer(c)
		c.JSON(http.StatusOK, data)
	})
	r.GET("/test_stream/:media", func(c *gin.Context) {
		getMedia(c)
	})
	r.Run("localhost:8080")
}

func duplicateServer(c *gin.Context) GetContentByChunkResponse {
	//This function will act as server here provide chunk data
	contentId := c.Param("id")
	log.Println("contentId", contentId)
	id, _ := strconv.ParseInt(contentId, 10, 64)
	data, info := getchunk(id)
	//log.Println("info", info)
	//c.Writer.Write(data)
	size := getMediaInfo()
	return GetContentByChunkResponse{
		Status:           SUCCESS,
		ContentId:        139,
		ChunkId:          id,
		ChunkSize:        info.Size(),
		TotalContentSize: size,
		ContentData:      data,
		Message:          "Success",
	}
}

func getchunk(partno int64) ([]byte, os.FileInfo) {
	log.Println("first chunk")
	//filename := fmt.Sprintf("filepart_%v", +partno)
	filename := fmt.Sprintf("filepart_%v", +partno)
	log.Println("filename", filename)
	f, _ := os.Open("./" + filename)
	info, _ := f.Stat()
	log.Println(info.Size())
	b1 := make([]byte, info.Size())
	n1, _ := f.Read(b1)
	fmt.Printf("%d bytes: %v\n", n1, info.Size())
	return b1, info
}

func createChunk() {
	//this function used to divide video as chunks
	fileToBeChunked := "./139.mp4" // change here!
	file, err := os.Open(fileToBeChunked)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer file.Close()

	fileInfo, _ := file.Stat()

	var fileSize int64 = fileInfo.Size()

	const fileChunk = 16 * (1 << 20) // 16 MB, change this to your requirement

	// calculate total number of parts the file will be chunked into

	totalPartsNum := uint64(math.Ceil(float64(fileSize) / float64(fileChunk)))

	fmt.Printf("Splitting to %d pieces.\n", totalPartsNum)

	for i := uint64(0); i < totalPartsNum; i++ {

		partSize := int(math.Min(fileChunk, float64(fileSize-int64(i*fileChunk))))
		partBuffer := make([]byte, partSize)

		file.Read(partBuffer)

		// write to disk
		fileName := "filepart_" + strconv.FormatUint(i, 10)
		_, err := os.Create(fileName)

		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// write/save buffer to disk
		ioutil.WriteFile(fileName, partBuffer, os.ModeAppend)

		fmt.Println("Split to : ", fileName)
	}

}

func getMediaInfo() int64 {
	f, _ := os.Open("./139.mp4")
	info, _ := f.Stat()
	//log.Println("info of whole video", info.Size(), info.Name())
	return info.Size()
}

func getMedia(c *gin.Context) {
	//log.Printf("GetMedia context %+v", c)
	//requestRange := c.GetHeader("Range")
	requestRange := c.GetHeader(header.Range)
	log.Println(" ___ Range from API", requestRange)
	id := c.Param("media")
	log.Println(" ___id from api", id)
	info := getMediaInfo()
	log.Println("&&&&&& Generate range function call &&&&")
	err, mediaRange := GenerateRange(info, requestRange)
	if err != nil {
		log.Printf("Error with range ", err.Error())
		c.Status(500)
	}
	rangeHeader := "bytes " + strconv.FormatInt(mediaRange.Start, 10) + "-" + strconv.FormatInt(mediaRange.End, 10) + "/" + strconv.FormatInt(mediaRange.TotalMediaSize, 10)
	c.Header(header.ContentRange, rangeHeader)
	c.Status(206)
	if mediaRange.End == 0 {
		return
	}
	//response.Header().Set("Accept-Ranges", "bytes")
	//c.Header("Transfer-Encoding","chunked")
	c.Header(header.ContentType, "video/mp4")
	//c.Header(header.AcceptRange,"bytes")
	c.Header(header.ContentLength, strconv.FormatInt(mediaRange.Length, 10))

	c.Stream(func(writer io.Writer) bool {

		err, code := StreamRange(writer, mediaRange)

		c.Status(code)
		if err != nil {
			log.Println(err)
			return false
		}

		return false
	})

}

func StreamRange(writer io.Writer, mediaRange Range) (err error, code int) {
	log.Println(" ************** inside Stream Range ******************")
	streamedBytes := int64(0)

	chunkSize := int64(16 * 1024 * 1024)
	i := 0
	for streamedBytes < mediaRange.Length {

		chunkLength := chunkSize
		log.Printf("chunkLEngth= %v,chunk size = %v, mediaRange.leng = %v,streamedBytes = %v", chunkLength, chunkSize, mediaRange.Length, streamedBytes)

		if chunkSize > mediaRange.Length-streamedBytes {
			chunkLength = mediaRange.Length - streamedBytes
		} else if mediaRange.Start+chunkLength > mediaRange.TotalMediaSize {
			chunkLength = mediaRange.TotalMediaSize - mediaRange.Start
		}
		var outputdata GetContentByChunkResponse

		urlforget := fmt.Sprintf("http://localhost:8080/server/%v", i)
		chunkData, err := http.Get(urlforget)
		log.Println("url =", urlforget)
		if err != nil {
			log.Println(err, "Problem getting chunk data")
			//return fmt.Errorf("problem striming chunks %s", err.Error()), http.StatusInternalServerError, w
		}
		//io.ReadWriteSeeker()
		//json.NewDecoder(chunkData.Body).Decode(outputdata)
		body, _ := ioutil.ReadAll(chunkData.Body)
		json.Unmarshal(body, &outputdata)
		log.Printf("chunkid = %v ,chunkLength = %v, Len of content = %v", outputdata.ChunkId, outputdata.ChunkSize, len(outputdata.ContentData))
		//io.ReadWriteSeeker()
		write, err := writer.Write(append(outputdata.ContentData))
		if nil != err {
			log.Printf("Error from streaming to client %s", err.Error())
			break
		}
		streamedBytes += chunkLength
		mediaRange.Start += chunkLength
		i++

		percent := streamedBytes / mediaRange.Length * 100
		log.Printf("length %d written %d streamed bytes %d %%%d", mediaRange.Length, write, streamedBytes, percent)

		if err != nil {

			log.Printf("%s ,error streaming to client", err)
			break
		}
	}

	log.Println(" ************** end of  Stream Range ******************")

	return nil, 206
}

func GenerateRange(info int64, rangeHeader string) (err error, mediaRange Range) {

	contentLength := info

	rangeSlice := rangeExpression.FindStringSubmatch(rangeHeader)

	if nil == rangeSlice {
		return fmt.Errorf("cannot parse range %s", rangeHeader), Range{}
	}

	//size, err := strconv.ParseInt(contentLength, 10, 64)
	size := contentLength
	if nil == rangeSlice {

		return fmt.Errorf("cannot parse range %s", rangeHeader), Range{}
	}

	if nil == rangeSlice {

		return fmt.Errorf("cannot parse range %s", rangeHeader), Range{}
	}

	startRange, err := strconv.ParseInt(rangeSlice[1], 10, 64)
	if err != nil {

		return fmt.Errorf("Error parsing range %s", rangeHeader), Range{}
	}

	endRange := int64(0)
	if len(rangeSlice[2]) > 0 {
		endRange, err = strconv.ParseInt(rangeSlice[2], 10, 64)
	} else {
		endRange = size - 1
	}
	if err != nil {

		return fmt.Errorf("Error parsing range %s", rangeHeader), Range{}
	}

	length := endRange + 1 - startRange

	return nil, Range{
		Start:          startRange,
		End:            endRange,
		Length:         length,
		TotalMediaSize: size,
	}
}
