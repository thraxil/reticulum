workflow "run go test on push" {
  on = "push"
  resolves = ["test"]
}

action "test" {
  uses = "./.github/actions/action-go-test"
}
