(package main)
(import "fmt")

; Return types go before parameter vectors; parameter types go before parameter names.
(defn factorial ^int64 [^int64 n]
  (if (<= n 1) 1 (* n (factorial (- n 1)))))

(defn greeting ^string [^string name]
  (let [prefix "Hello "] (+ prefix name)))

(defn half ^float64 [^float64 x] (/ x 2.0))

(defn main []
  ; fn return types go on their parameter vectors.
  (let [x 10 add (fn ^int64 [^int64 y] (+ x y))]
    (fmt.Println (add 5)))
  (fmt.Println (factorial 5) (greeting "John") (half 5.0))
  (fmt.Println ^string (if (> 2 1) "yes" "no"))
  (fmt.Println ^int64 (let [x 3 x (+ x 1)] (* x x)))
  (when (and true (not false)) (fmt.Println "when"))
  (if false (fmt.Println "bad") (fmt.Println "else")))
