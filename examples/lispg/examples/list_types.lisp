(package main)
(import "fmt")

(defn ^[string] words [] ["hello" "world"])
(defn ^[float64] missing [] (when false [1.5]))
(defn ^(fn [int64] int64) incrementer [] (fn ^int64 [^int64 x] (+ x 1)))

(defn main []
  (fmt.Println (first [1 2]) (rest [1 2]) (empty? ^[int64] []))
  (fmt.Printf "%T %T %T %T\n" [1 2] [1.5 2.5] ["a" "b"] [true false])
  (fmt.Println (cons "hi" (words)) (cons 1.5 [2.5]))
  (fmt.Println (rest [1]) (rest ^[int64] []))
  (let [^[int64] xs []] (fmt.Println (empty? xs)))
  (fmt.Println (missing) ((incrementer) 4))
  (fmt.Println ^[[int64]] [[1 2] []])
  (let [x 7] (fmt.Println [x (+ x 1)]))
  (let [xs [1 2 3] tail (rest xs) a (cons 9 tail) b (cons 8 tail)]
    (fmt.Println xs tail a b))
  (fmt.Println ^[int64] (cons 1 [])))
