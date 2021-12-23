import re

# exports resource, operation and route path
# usage:
# python export.py | sort -t, -k 2 -k 1 > permission.csv

with open("route.go", "r") as f:
  txt = f.read()
  #print(txt)
  r = re.compile("^[ \t]*(((\w+\.(GET|POST|DELETE|PUT))|(ctrl\.Register)).*)$",re.MULTILINE)
  x = re.findall(r, txt)
  for g in x:
    g = g[0]
    op = "all"
    path = ""
    res = ""
    #print("line: %s" % g)
    if "g.Group" in g:
      op = "all"
      #print("op all")
      g = g.replace("ctrl.Register(","")
      #print("replaced line [%s]" % g)

    else:
      o = re.search("\w+\.(\w+).*", g)
      #print("op [%s]" % o.group(1))
      op = o.group(1)
    p = re.search(r"\(\"([^\"].*?)\".*", g)
    #print("path [%s]" % p.group(1))
    path = p.group(1)
    if op == 'all':
      path = "/" + path

    r = re.search(r"\\?(\w+)", path)
    #print("res [%s]" % r.group(1))
    res = r.group(1)

    ac = re.search(r"ac\(\"(\w+)\",\s*\"(\w+)\"\)", g)
    if ac:
      #print("replaced [%s, %s]" % (ac.group(1), ac.group(2)))
      res = ac.group(1)
      method = op
      op = ac.group(2)


    #print("op %s res %s path %s" % (op, res, path))
    if op == 'all':
      for op,method in [('read','GET'),('create','POST'),('delete','DELETE'),('update','PUT')]:
        if method == 'GET':
          print("%s,%s,%s %s" % (op, res, method, path))
          print("%s,%s,%s %s/:id" % (op, res, method, path))
        elif method == 'POST':
          print("%s,%s,%s %s" % (op, res, method, path))
        elif method == 'DELETE':
          print("%s,%s,%s %s/:id" % (op, res, method, path))
        elif method == 'PUT':
          print("%s,%s,%s %s/:id" % (op, res, method, path))
    elif op == 'GET':
        print("%s,%s,%s %s" % ('read', res, op, path))
    elif op == 'POST':
        print("%s,%s,%s %s" % ('create', res, op, path))
    elif op == 'DELETE':
        print("%s,%s,%s %s" % ('delete', res, op, path))
    elif op == 'PUT':
        print("%s,%s,%s %s" % ('update', res, op, path))
    else:
        print("%s,%s,%s %s" % (op, res, method, path))

