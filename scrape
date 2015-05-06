#! /usr/bin/node

var request = require("request"),
    fs = require("fs"),
    argv = require('yargs').argv,
    html_strip = require('htmlstrip-native'),
    keys = require("./keys.private.json"),
    page_size = 200,
    url = "http://content.guardianapis.com/search?show-fields=body&page-size=100",
    query = "";

argv.q = argv.q || argv.query;
if (argv.q){
    query = argv.q.replace(" ", "+");
} else {
    console.log("Query parameter (-q) missing.\nUsage example:\tscrape -q 'David Cameron'")
    process.exit(code=0);
}

url = url + "&api-key=" + keys.key + "&q=" + query;
scrape(url)

function scrape(url) {
    request(url, function(err, res, body){
        var obj = JSON.parse(body),
            results = obj.response.results,
            stripped, 
            output;
        results.length < 1 ? error("Error. No results.") : false;
        output = conc(results)
        stripped = strip(output)
        console.log(stripped)
    })
}

function conc(results) {
    var output = "";
    for (var i in results){
        if (results[i].fields && results[i].fields.body){
            output += results[i].fields.body
        }
    }
    return output;
}

function strip(html) {
    var options = {
        include_script : false,
        include_style : false,
        compact_whitespace : true
    }
    return html_strip.html_strip(html, options);
}

function error(str) {
    console.log(str)
    process.exit(code=1)
}