# Push to all remotes
push:
  git push && git push local && git push personal


# Git Diff All Changed and Stagged
diff U="":
  @echo "-----------------Changed-------------------"
  @git --no-pager diff {{U}} -- \
  '**/*.go' \
  ':(exclude)**/*.md' \
  ':(exclude)**/justfile' \
  ':(exclude)**/*.just' \
  ':(exclude)**/go.mod' \
  ':(exclude)**/go.sum'
  @echo "-----------------Staged-------------------"
  @git --no-pager diff --staged {{U}} -- \
  '**/*.go' \
  ':(exclude)**/*.md' \
  ':(exclude)**/justfile' \
  ':(exclude)**/*.just' \
  ':(exclude)**/go.mod' \
  ':(exclude)**/go.sum'

# Git Diff Staged Files
staged U="":
  @git --no-pager diff --staged {{U}} -- \
  '**/*.go' \
  ':(exclude)**/*.md' \
  ':(exclude)**/justfile' \
  ':(exclude)**/*.just' \
  ':(exclude)**/go.mod' \
  ':(exclude)**/go.sum'

# Git Diff Changed Files
changed U="":
  @git --no-pager diff {{U}} -- \
  '**/*.go' \
  ':(exclude)**/*.md' \
  ':(exclude)**/justfile' \
  ':(exclude)**/*.just' \
  ':(exclude)**/go.mod' \
  ':(exclude)**/go.sum'

# Git Diff to Main
diff-main U="":
  @git --no-pager diff main {{U}} -- \
  '**/*.go' \
  ':(exclude)**/*.md' \
  ':(exclude)**/justfile' \
  ':(exclude)**/*.just' \
  ':(exclude)**/go.mod' \
  ':(exclude)**/go.sum'

