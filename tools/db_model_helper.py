import os

update_files = {
  "investment_holdings.go",
  "trade_order.go",
  "investment_trade.go",
  "adjusted_price.go"
}

os.chdir("./internal/db/models/postgres/public/model/")
files = os.listdir()
for file in files:
  # check if any of the update file names
  # exist in the current file path
  for u in update_files:
    if u not in file:
      continue
    print("rewriting", file)
    f = open(file)
    contents = f.read()
    f.close()
    if "float64" in contents:
      contents = contents.replace("float64", "decimal.Decimal")
      if "\"time\"" in contents:
        contents = contents.replace("\"time\"", "\"time\"\n\n\t\"github.com/shopspring/decimal\"")
      else:
        contents = contents.replace("package model", "package model\n\nimport (\n\t\"github.com/shopspring/decimal\"\n)")
      f = open(file, 'w')
      f.write(contents)
      f.close()
