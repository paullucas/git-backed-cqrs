#!/usr/bin/env bash

# Create a Todo List
curl --data "name=test" localhost:8080/createTodoList
curl --data "name=test2" localhost:8080/createTodoList

# Invalid request
# curl --data "1234=5678" localhost:8080/createTodoList
