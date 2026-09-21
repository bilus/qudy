(package main)
(import "fmt" "strings")

(defn main []
  (let [a 10 b "John"]
       (fmt.Printf "Hello %s (%d)\n" (strings.ToUpper b) a)))
