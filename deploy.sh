git add -A
if ! git diff-index --quiet HEAD; then
  git commit -m "Message here"
  git push origin main
fi
