#!/usr/bin/env bash

download () {
    date
    curl "https://dropbike.herokuapp.com/v3/bikes?lat=49.2606&lng=-123.2460" |
        gzip -c > data/bikes-$(date -Iseconds).json.gz
}

export -f download

watch -n 30 bash -c download

