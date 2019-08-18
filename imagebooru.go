package imagebooru

import (
	"encoding/xml"
	"errors"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/Stachio/go-printssx"
)

// Printer - Generic printer object provided by stachio/printerssx
var Printer = printssx.New("IMGBOORU", log.Println, log.Printf, printssx.Subtle, printssx.Subtle)

type Post struct {
	XMLName   xml.Name `xml:"post"`
	FileURL   string   `xml:"file_url,attr"`
	SampleURL string   `xml:"sample_url,attr"`
	Source    string   `xml:"source,attr"`
	ID        uint64   `xml:"id,attr"`
	Tags      string   `xml:"tags,attr"`

	// Useful data types
	Bytes []byte
	Img   image.Image
	PHash uint64
}

type Page struct {
	XMLName xml.Name `xml:"posts"`
	Count   uint64   `xml:"count,attr"`
	Offset  uint64   `xml:"offset,attr"`
	Posts   []Post   `xml:"post"`
}

type Tag struct {
	XMLName xml.Name `xml:"tag"`
	Type    int      `xml:"type,attr"`
	Count   int64    `xml:"count,attr"`
	Name    string   `xml:"name,attr"`
	ID      uint64   `xml:"id,attr"`
}

type Tags struct {
	XMLName xml.Name `xml:"tags"`
	Type    string   `xml:"type,attr"`
	Tags    []Tag    `xml:"tag"`
}

type ImageBooru struct {
	url     string
	name    string
	postCap uint64
	pageCap uint64
}

/*
func (post *Post) GetPerceptionHash() (pHash uint64, err error) {
	resp, err := http.Get(post.FileURL)
	if err != nil {
		return
	}

	return
}
*/

func getDBNameFromURL(url string) (name string) {
	name = strings.ToLower(url)
	http := "http://"
	https := "https://"
	if name[:len(http)] == http {
		name = name[len(http):]
	} else if name[:len(https)] == https {
		name = name[len(https):]
	}

	for i, c := range name {
		if c == '/' {
			name = name[:i]
			break
		}
	}

	name = strings.Replace(name, ".", "", -1)
	return
}

//New - Returns ImageBooru object
func New(url string, postCap uint64) (ib *ImageBooru) {
	ib = &ImageBooru{url: url, name: getDBNameFromURL(url), postCap: postCap}
	return
}

type Browser struct {
	ib    *ImageBooru
	tags  []string
	pages []*Page
}

func (ibb *Browser) GetIBName() string {
	return ibb.ib.name
}

// GetPage - Allows
func (ibb *Browser) GetPage(pageID uint64) (page *Page, err error) {
	Printer.Printf(printssx.Subtle, "Querying page:%d with tags \"%s\"", pageID, strings.Join(ibb.tags, ","))

	if uint64(len(ibb.pages)) >= (pageID+1) && ibb.pages[pageID] != nil {
		page = ibb.pages[pageID]
		return
	}

	page = &Page{}
	tagStr := ConvertTags(ibb.tags)
	url := fmt.Sprintf("%s/index.php?page=dapi&s=post&q=index&pid=%d&tags=%s", ibb.ib.url, pageID, tagStr)
	err = GetXML(url, page)
	if err != nil {
		return nil, err
	}

	for uint64(len(ibb.pages)) < (pageID + 1) {
		ibb.pages = append(ibb.pages, nil)
	}
	ibb.pages[pageID] = page
	return
}

// GetBrowser - Not now
func (ib *ImageBooru) GetBrowser(tags []string) (ibb *Browser, err error) {
	ibb = &Browser{ib: ib, tags: tags}
	if err != nil {
		return nil, err
	}

	return
}

//NewWithResearch - Returns ImageBooru object with researched postcap
func NewWithResearch(url string) (ib *ImageBooru, err error) {
	ib = &ImageBooru{url: url, name: getDBNameFromURL(url)}
	ibb, err := ib.GetBrowser([]string{""})
	if err != nil {
		return nil, err
	}

	// Page 1 is used because offset is set to booru cap
	page, err := ibb.GetPage(1)
	if err != nil {
		return nil, err
	}

	ib.postCap = page.Offset
	Printer.Println(printssx.Subtle, "Booru cap count:", ib.postCap)
	return
}

// GetXML - Return an xml object per url HTTP request
func GetXML(url string, v interface{}) (err error) {
	// Get the reponse from the provided url
	resp, err := http.Get(url)
	if err != nil {
		return
	}

	// Convert http request to bytes
	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	// Close the http response body
	err = resp.Body.Close()
	if err != nil {
		return
	}

	// Convert the bytes to an ImageBooru.Page XML object
	err = xml.Unmarshal(bytes, v)
	return
}

// GetName - Self explanatory
func (ib *ImageBooru) GetName() string {
	return ib.name
}

// ConvertTags - Convert tags []strings to string
// Each tags is escaped via url.QueryEscape
func ConvertTags(tags []string) string {
	tagStr := url.QueryEscape(strings.Join(tags, " "))
	return tagStr
}

func (ibb *Browser) GetPost(offset uint64) (post *Post, err error) {
	// Get the current page ID
	pageID := offset / ibb.ib.postCap

	// Get the post id as an offset of the page
	offset = (offset + 1) % ibb.ib.postCap
	if offset == 0 {
		offset = ibb.ib.postCap - 1
	} else {
		offset = offset - 1
	}

	page, err := ibb.GetPage(pageID)
	if err != nil {
		return nil, err
	}

	if offset >= uint64(len(page.Posts)) {
		err = fmt.Errorf("Offset [%d] > returned posts length [%d]", offset, len(page.Posts))
		return
	}

	post = &page.Posts[offset]
	Printer.Printf(printssx.Subtle, "Returning %d\n", ((pageID * ibb.ib.postCap) + offset))
	//ibPageOut = ibPageIn
	return
}

func (ibb *Browser) GetIBPost(postId string) (post *Post, err error) {
	post, err = ibb.ib.GetPost(postId)
	return
}

func (ib *ImageBooru) GetPost(postID string) (post *Post, err error) {
	Printer.Printf(printssx.Subtle, "Querying pageID:%s", postID)

	page := &Page{}
	url := fmt.Sprintf("%s/index.php?page=dapi&s=post&q=index&id=%s", ib.url, postID)
	err = GetXML(url, page)
	if err != nil {
		return nil, err
	}

	for len(page.Posts) < 1 {
		return nil, Printer.Errorf("Somehow pageID:%s return nothing?", postID)
	}
	post = &page.Posts[0]
	return
}

func (ibb *Browser) GetTag(tagName string) (tag *Tag, err error) {
	tags := &Tags{}
	url := ibb.ib.url + "/index.php?page=dapi&s=tag&q=index&name=" + url.QueryEscape(tagName)
	err = GetXML(url, tags)
	if err != nil {
		if err.Error() == "expected element type <tags> but have <tag>" {
			return nil, nil
		}
		return nil, err
	}
	tag = &tags.Tags[0]

	return
}

const httpTimeout = 5 //seconds
var httpTimeoutError = errors.New("Http request timed out")

func (post *Post) loadImage(imageType, imgURL string) (err error) {
	Printer.Printf(printssx.Subtle, "Downloading image %s:%s\n", imageType, imgURL)

	if imgURL[:2] == "//" {
		imgURL = "https:" + imgURL
	}
	imgExt := path.Ext(imgURL)

	var decoder func(io.Reader) (image.Image, error)
	if imgExt == ".png" {
		decoder = png.Decode
	} else if imgExt == ".jpg" || imgExt == ".jpeg" {
		decoder = jpeg.Decode
	} else if imgExt == ".gif" {
		decoder = gif.Decode
	} else {
		err = fmt.Errorf("Unexpected filetype \"%s\"", imgExt)
		//fmt.Println(err)
		return
	}

	//fmt.Printf("Pulling image index:%d, ID:%d, URL:%s, Ext:%s\n", index, post.ID, imgURL, imgExt)
	client := &http.Client{
		Timeout: httpTimeout * time.Second,
	}
	resp, err := client.Get(imgURL)
	if err != nil {
		Printer.Println(printssx.Subtle, "Failed to downloaded image")
		//fmt.Println(err)
		return
	}

	Printer.Println(printssx.Subtle, "Decoding image")
	post.Img, err = decoder(resp.Body)

	if err != nil {
		return
	}

	return
}

func (post *Post) LoadImage() (err error) {
	err = post.loadImage("booru", post.FileURL)
	if err != nil {
		Printer.Println(printssx.Subtle, "Booru download failed, attempting source")
		err2 := post.loadImage("source", post.Source)
		if err2 != nil {
			err = fmt.Errorf("%s\n%s", err.Error(), err2.Error())
		} else {
			err = nil
		}
	}

	return
}
