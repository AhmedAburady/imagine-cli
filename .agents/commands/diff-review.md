## Perform git diff code review

Steps:

1. run `git status` to understand what's staged and what is in changes
2. `git --no-pager diff --staged --stat` use `--staged` based on whether the files staged or not
3. `git --no-pager diff --staged -- . ':(exclude)*.md' ':(exclude)*.lock'` to view code diffs
4. Review the code:
   - Spot any breaking changes and report
   - Verify the code is maintainable, efficient and performant
   - Spot any inconsistency with existing patterns of styling or code style
   - spot any potential performance or security issues
   - spot any potential sepration of concerns
5. Report your findings
6. suggest solutions or fixes if issues are found
7. confirm the code is 100% production ready to commit
