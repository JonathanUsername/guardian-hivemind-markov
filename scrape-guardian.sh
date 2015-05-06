#!/bin/bash
# echo "Simple guardian scraper - first parameter is your API key, second is the search query, ie. guardian-scraper hgu54hng5n 'David Cameron'"
curl -s "http://content.guardianapis.com/search?show-fields=body&page-size=200&api-key=$1&q=$2" | jq .response.results[].fields.body | sed 's/<[^>]\+>/ /g'
