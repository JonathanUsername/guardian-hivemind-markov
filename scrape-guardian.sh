#!/bin/bash
echo "Simple guardian scraper - first parameter is your API key, second is the search query, ie. guardian-scraper hgu54hng5n 'David Cameron'"
curl "http://content.guardianapis.com/search?show-fields=body&api-key=$1&q=$2" | jq .response.results[].fields.body | sed 's/<[^>]\+>/ /g'
