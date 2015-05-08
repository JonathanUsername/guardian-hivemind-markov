
// The Markov chain section of this code is taken from the "Generating arbitrary text"
// codewalk that can be found here: http://golang.org/doc/codewalk/markov/

package main

import (
    "flag"
    "sync"
    "bufio"
    "fmt"
    "io"
    "math/rand"
    "strings"
    "time"
    "net/http"
    "strconv"
    "github.com/jmoiron/jsonq"
    "encoding/json"
    "io/ioutil"
    "os"
    "regexp"
)

var keyfile string

// Prefix is a Markov chain prefix of one or more words.
type Prefix []string

// String returns the Prefix as a string (for use as a map key).
func (p Prefix) String() string {
    return strings.Join(p, " ")
}

// Shift removes the first word from the Prefix and appends the given word.
func (p Prefix) Shift(word string) {
    copy(p, p[1:])
    p[len(p)-1] = word
}

// Chain contains a map ("chain") of prefixes to a list of suffixes.
// A prefix is a string of prefixLen words joined with spaces.
// A suffix is a single word. A prefix can have multiple suffixes.
type Chain struct {
    chain     map[string][]string
    prefixLen int
}

// NewChain returns a new Chain with prefixes of prefixLen words.
func NewChain(prefixLen int) *Chain {
    return &Chain{make(map[string][]string), prefixLen}
}

// Build reads text from the provided Reader and
// parses it into prefixes and suffixes that are stored in Chain.
func (c *Chain) Build(r io.Reader) {
    br := bufio.NewReader(r)
    p := make(Prefix, c.prefixLen)
    for {
        var s string
        if _, err := fmt.Fscan(br, &s); err != nil {
            break
        }
        key := p.String()
        c.chain[key] = append(c.chain[key], s)
        p.Shift(s)
    }
}

// Generate returns a string of at most n words generated from Chain.
func (c *Chain) Generate(n int) string {
    p := make(Prefix, c.prefixLen)
    var words []string
    for i := 0; i < n; i++ {
        choices := c.chain[p.String()]
        if len(choices) == 0 {
            break
        }
        next := choices[rand.Intn(len(choices))]
        words = append(words, next)
        p.Shift(next)
    }
    return strings.Join(words, " ")
}

func getFields(stringbody string, field string) string {
    results, err := fromJson(stringbody).Array("response", "results")
    check(err)
    var wholebody string
    regex, _ := regexp.Compile("<[^>]+>")
    for _, each := range results {
        body := each.(map[string]interface{})["fields"].(map[string]interface{})[field]
        cleaned := regex.ReplaceAllString(body.(string), " ")
        wholebody += cleaned
    }
    return wholebody
}

func Scrape(query string, wg *sync.WaitGroup) string {
    url := fmt.Sprintf("http://content.guardianapis.com/search?show-fields=body,headline&page-size=200&api-key=%s&q=%s", getKey(), query)
    resp, err := http.Get(url)
    check(err)
    defer resp.Body.Close()

    // Why can't I find a go-based json parser that works with io.Reader rather than strings? For purity's sake I'm doing this rather than piping to jq, but I think this is a bottleneck...
    body, err := ioutil.ReadAll(resp.Body)

    wg.Done()
    return string(body)
}

type article_response struct {
    Headline    string
    Body        string
}

func CreateArticle(query string, wordL int, prefixL int) article_response {
    rand.Seed(time.Now().UnixNano()) // Seed the random number generator.
    var wg sync.WaitGroup         // Create waitgroup for scrape process
    wg.Add(1)
    scrape := Scrape(query, &wg)
    wg.Wait()

    // text = buildPart("headline", scrape, 2, 20)   // this turned out to be a bit too random

    // build headline
    arr, _ := fromJson(scrape).Array("response", "results")
    random_index := rand.Intn(len(arr))
    headline, _ := fromJson(scrape).String("response", "results", strconv.Itoa(random_index), "fields", "headline")

    // build body
    body := buildPart("body", scrape, prefixL, wordL)

    artr := article_response{Headline: headline, Body: body}
    return artr
}

func buildPart(part string, scrape string, prefixL int, wordL int) string {
    c := NewChain(prefixL)
    cleanbody := getFields(scrape, part)
    b := io.Reader(strings.NewReader(cleanbody))
    c.Build(b)
    text := c.Generate(wordL)
    return text
}

func main() {
    flag.StringVar(&keyfile, "keyfile", "", "File that stores Guardian Developer API key.")
    flag.Parse()
    if keyfile == "" {
        fmt.Println("Keyfile argument (--keyfile) missing. Assuming default path.")
        keyfile = "./keys.private.json"
    }
    fmt.Printf("Keyfile: '%s'\n", keyfile)
    listen()
}

func handler(w http.ResponseWriter, r *http.Request) {
    fmt.Println("requestURI:  " + string(r.RequestURI))
    q := r.URL.Query().Get("q")
    q = strings.Replace(q," ","+",-1)
    fmt.Println(q)
    wordL, err := strconv.Atoi(r.URL.Query().Get("wl"))
    if err != nil {
        wordL = 100
    }
    prefixL, err := strconv.Atoi(r.URL.Query().Get("pl"))
    if err != nil {
        prefixL = 2
    }
    article := CreateArticle(q, wordL, prefixL)
    jsonresp, err := json.Marshal(article)
    check(err)
    fmt.Println("returning!")
    fmt.Fprintf(w, "%s", jsonresp)
}

func listen() {
    fmt.Printf("Listening on port 8080\n\n")
    http.HandleFunc("/write", handler)
    http.ListenAndServe(":8080", nil)
}

func getKey() string {
    file, e := ioutil.ReadFile(keyfile)
    if e != nil {
        fmt.Printf("File error: %v\n", e)
        os.Exit(1)
    }
    apikey, err := fromJson(string(file)).String("key")
    check(err)
    return apikey
}

func fromJson(js string) *jsonq.JsonQuery {
    // usage: var, err := fromJson(json).String("value", "nestedvalue", "somearray, "0")
    data := map[string]interface{}{}
    dec := json.NewDecoder(strings.NewReader(js))
    dec.Decode(&data)
    jq := jsonq.NewQuery(data)
    return jq
}

func check(err error) {
    if err != nil {
        panic(err)
    }
}