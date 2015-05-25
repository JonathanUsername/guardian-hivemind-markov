
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
    "encoding/json"
    "io/ioutil"
    "os"
    "regexp"
    "net/url"
    "github.com/jmoiron/jsonq"
    "github.com/nu7hatch/gouuid"
    "gopkg.in/mgo.v2"
)

var KEYFILE string
var DEFAULT_WORD_LENGTH int = 2000
var DEFAULT_PREFIX_LENGTH int = 2

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
        // fmt.Println(key)
        c.chain[key] = append(c.chain[key], s)
        p.Shift(s)
    }
}

// Generate returns a string of at most n words generated from Chain.
func (c *Chain) Generate(n int, single bool) string {
    p := make(Prefix, c.prefixLen)
    var words []string
    // select first word randomly
    var first_prefix string
    for first_prefix, _ = range c.chain {
        if len(first_prefix) > 6 && first_prefix[:6] == "<end/>"{
            // first_prefix = strings.Replace(first_prefix,"<end/>","",-1)
            arr := strings.Split(first_prefix, " ")
            for i:=0;i<c.prefixLen;i++ {
                p[i] = arr[i]
            }
            words = append(words, first_prefix)
            // fmt.Println("First prefix of chain:")
            // fmt.Println(p.String())
            break
        }
    }
    // generate it
    for i := 0; i < n; i++ {
        var word string = p.String()
        choices := c.chain[word]
        // if i < 3 {
        //     for j :=0; j < len(choices); j++ {
        //         fmt.Println("CHOICES:"+string(j))
        //         fmt.Println(choices[j])
        //     }
        // }
        if len(choices) == 0 {
            fmt.Println("No choices. Breaking.")
            break
        }
        next := choices[rand.Intn(len(choices))]
        if single && strings.Contains(next, "<end/>") {
            fmt.Println("Found end tag. Breaking.")
            break
        }
        words = append(words, next)
        if i == n-1 {
            fmt.Println("Reached word limit. Breaking.")
            words = append(words, "[...]")
        }
        p.Shift(next)
    }
    return strings.Join(words, " ")
}

// func hasClosingSuffix(str string) bool {
//     return strings.ContainsAny(".?!", str)
// }

func getFields(stringbody string, field string) string {
    results, err := fromJson(stringbody).Array("response", "results")
    check(err)
    var wholebody string
    strip_tags_regex, _ := regexp.Compile("<[^>]*>")
    for _, each := range results {
        if body, ok := each.(map[string]interface{})["fields"].(map[string]interface{})[field]; ok {
            var chunk string = body.(string)
            if field != "body" {
                body = strip_tags_regex.ReplaceAllString(body.(string), " ")
            }
            chunk += " <end/>"
            wholebody += chunk
        }
    }
    return wholebody
}

func Scrape(query string, wg *sync.WaitGroup) string {
    query = url.QueryEscape(query)
    fmt.Println(query)
    apiurl := fmt.Sprintf("http://content.guardianapis.com/search?show-fields=body,headline,trailText,main&page-size=200&api-key=%s&q=%s", getKey(), query)
    resp, err := http.Get(apiurl)
    check(err)
    defer resp.Body.Close()

    // Why can't I find a go-based json parser that works with io.Reader rather than strings? For purity's sake I'm doing this rather than piping to jq, but I think this is a bottleneck...
    body, err := ioutil.ReadAll(resp.Body)

    wg.Done()
    return string(body)
}

type article_response struct {
    Id          string `json:"id" bson:"_id,omitempty"`
    Article     article `json:"article" bson:"article"`
    Cache       string `json:"cache" bson:"cache"`
}

type article struct {
    Headline    string
    Body        string
    Trailtext   string
    Main        string
}

func CreateArticle(query string, wordL int, prefixL int) article_response {
    rand.Seed(time.Now().UnixNano()) // Seed the random number generator.
    var wg sync.WaitGroup         // Create waitgroup for scrape process
    wg.Add(1)
    scrape := Scrape(query, &wg)
    wg.Wait()
    // arr, _ := fromJson(scrape).Array("response", "results")
    // random_index := rand.Intn(len(arr))
    // randomly_selected_headline, _ := fromJson(scrape).String("response", "results", strconv.Itoa(random_index), "fields", "headline")
    headline := buildPart("headline", scrape, prefixL, wordL, true)
    trailtext := buildPart("trailText", scrape, prefixL, wordL, true)
    body := buildPart("body", scrape, prefixL, wordL, true)
    main := buildPart("main", scrape, prefixL, wordL, true)
    art := article{Headline: headline, Body: body, Trailtext: trailtext, Main: main}
    resp := cacheArticle(art)
    return resp
}

func cacheArticle(article article) article_response {
    fmt.Printf("Caching")
    session, err := mgo.Dial("mongodb://localhost")
    check(err)
    c := session.DB("markov").C("cache")
    newu, err := uuid.NewV4()
    check(err)
    uuid := newu.String()
    cached := article_response{Article: article, Cache: uuid}
    ee := c.Insert(cached)
    check(ee)
    return cached
}

func buildPart(part string, scrape string, prefixL int, wordL int, single_sentence bool) string {
    fmt.Println("Building " + part)
    c := NewChain(prefixL)
    cleanbody := getFields(scrape, part)
    b := io.Reader(strings.NewReader(cleanbody))
    c.Build(b)
    text := c.Generate(wordL, single_sentence)
    return text
}

func main() {
    flag.StringVar(&KEYFILE, "keyfile", "", "File that stores Guardian Developer API key.")
    flag.Parse()
    if KEYFILE == "" {
        fmt.Println("Keyfile argument (--keyfile) missing. Assuming default path.")
        KEYFILE = "./keys.private.json"
    }
    fmt.Printf("Keyfile: '%s'\n", KEYFILE)
    listen()
}

func handler(w http.ResponseWriter, r *http.Request) {
    fmt.Println("requestURI:  " + string(r.RequestURI))
    q := r.URL.Query().Get("q")
    q = strings.Replace(q," ","+",-1)
    fmt.Println(q)
    wordL, err := strconv.Atoi(r.URL.Query().Get("wl"))
    if err != nil {
        wordL = DEFAULT_WORD_LENGTH
    }
    prefixL, err := strconv.Atoi(r.URL.Query().Get("pl"))
    if err != nil {
        prefixL = DEFAULT_PREFIX_LENGTH
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
    file, e := ioutil.ReadFile(KEYFILE)
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