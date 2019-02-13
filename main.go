package main

import (
	"bytes"
	"fmt"
	"golang.org/x/net/html"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
)

const URL = "https://medium.com/search/posts"
const KEYWORD = "kubernetes"
const GOPATH="."

var nblinks = map[string]bool{"10": true, "20": true, "30": true}

type Links struct {
	Title string
	Link  string
}

func main() {
	//ignore ids could be use later for paging
	ignoreIds := make([]string, 0)
	body := requestPage("q", KEYWORD, "count", "20")
	links := readAndParse(body, &ignoreIds)
	distribute(&ignoreIds, links, KEYWORD)
}

//GET the url
//Set the User-Agent
//Return the body
func requestPage(vals ...string) string {
	client := &http.Client{}

	req, err := http.NewRequest("GET", URL, nil)
	errHandling(err)

	req.URL.RawQuery = addQueryParams(req, vals...)
	fmt.Println(req.URL.String())

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/70.0.3538.110 Safari/537.36")
	resp, err := client.Do(req)
	errHandling(err)
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	errHandling(err)
	return string(body)
}

//HTTP handler for a route
//Return a template that contains an anonymous struct
func handler(w http.ResponseWriter, r *http.Request, ignoreIds *[]string, links []Links, keyword string, nb string) {
	tmpl := template.Must(template.ParseFiles(GOPATH+"/tmpl/index.html"))
	data := struct {
		PageTitle string
		Links     []Links
		Keyword   string
		Nb        string
	}{
		PageTitle: "Medium Searching Box",
		Links:     links,
		Keyword:   keyword,
		Nb:        nb,
	}
	tmpl.Execute(w, data)
}

func distribute(ignoreIds *[]string, links []Links, keyword string) {
	//Route: /
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		nb := "10"
		if len(r.Form) != 0 {
			nb = r.Form["nb"][0]
			if nblinks[nb] != true {
				fmt.Printf("[x - distribute] Bad number asked %s => 10 \n", nb)
				nb = "10"
			}
			keyword = r.Form["value"][0]
		}

		if len(keyword) < 3 || len(keyword) > 30 {
			keyword = KEYWORD
		}
		body := requestPage("q", keyword, "count", nb)
		links = readAndParse(body, ignoreIds)

		handler(w, r, ignoreIds, links, keyword, nb)
	})
	//Static route for assets
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(GOPATH+"/tmpl/static"))))
	//Listen and server on port 8080
	log.Fatal(http.ListenAndServe(":8080", nil))
}

//The battlefield <> read and parse the body
func readAndParse(res string, ignoreIds *[]string) []Links {
	b := bytes.NewBufferString(res)
	links := make([]string, 0) //Make a slice <> undefined link in there
	titles := make([]string, 0)
	page := html.NewTokenizer(b)
	for {
		tokenType := page.Next()
		if tokenType == html.ErrorToken {
			break
		}
		token := page.Token()
		if token.DataAtom.String() == "div" {
			for _, attr := range token.Attr {
				if attr.Key == "class" && attr.Val == "postArticle-content" {
					tokenType := page.Next()
					if tokenType == html.ErrorToken {
						break
					}

					token = page.Token()
				}
			}

			//Get the links inside the appropriate div
			if token.DataAtom.String() == "a" {
				for _, attr := range token.Attr {
					if attr.Key == "href" {
						links = append(links, attr.Val)
					}
					if attr.Key == "data-post-id" {
						*ignoreIds = append(*ignoreIds, attr.Val)
					}
				}
			}
		}
		//Get content within h3 with the class graf--title
		if token.DataAtom.String() == "h3" {
			for _, attr := range token.Attr {
				if strings.Contains(attr.Val, "graf--title") {
					titles = append(titles, getContentWithinTag(page))
				}
			}
		}
	}
	//Goroutine is our friends
	linksChan := make(chan []Links)
	go makeLinks(linksChan, links, titles)

	//Wait for something <> blocking
	return <-linksChan
}

//Goroutine func that compose the slice of links a send it to the channel
func makeLinks(linksChan chan []Links, links []string, titles []string) {
	linksStruct := make([]Links, 0)
	for k, link := range links {
		if k < len(titles) {
			linksStruct = append(linksStruct, Links{titles[k], link})
		}
	}
	//Send it the channel
	linksChan <- linksStruct
}

//Recursive func to get the content within a tag
func getContentWithinTag(page *html.Tokenizer) string {
	tokenType := page.Next()
	if tokenType == html.TextToken {
		return page.Token().Data
	}
	return getContentWithinTag(page)
}

//Prepare the query params
//even are keys
//odd are values
func addQueryParams(req *http.Request, vals ...string) string {
	if len(vals)%2 != 0 {
		log.Fatal("[ERR - addQueryParams] We need a key value pair for vals")
	}
	q := req.URL.Query()
	for k := range vals {
		if k%2 == 0 {
			q.Add(vals[k], vals[k+1])
		}
	}
	return q.Encode()
}

//Error handling
func errHandling(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
