#!/bin/bash
cloc --exclude-dir vendor,proto --match-f=".go$"  .
