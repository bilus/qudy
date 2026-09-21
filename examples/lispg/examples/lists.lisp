;; Lists and higher-order functions in Lisp. This example demonstrates the use
;; of lists, map, reduce, and filter functions.
(package main)
(import "fmt")

;; These functions are ordinary Lisp. The compiler only supplies the list primitives.
(defn map ^[int64] [^(fn [int64] int64) f ^[int64] xs]
      (if (empty? xs)
          []
          (cons (f (first xs)) (map f (rest xs)))))

(defn reduce ^int64 [^(fn [int64 int64] int64) f ^int64 initial ^[int64] xs]
      (if (empty? xs)
          initial
          (reduce f (f initial (first xs)) (rest xs))))

(defn filter ^[int64] [^(fn [int64] bool) predicate ^[int64] xs]
      (if (empty? xs)
          []
          (if (predicate (first xs))
              (cons (first xs) (filter predicate (rest xs)))
              (filter predicate (rest xs)))))

(defn square ^int64 [^int64 x] (* x x))
(defn add ^int64 [^int64 x ^int64 y] (+ x y))

(defn main []
  (let [xs [1 2 3 4]
       even (fn ^bool [^int64 x] (= (% x 2) 0))]
       (fmt.Println "map:" (map square xs))
       (fmt.Println "reduce:" (reduce add 0 xs))
       (fmt.Println "filter:" (filter even xs))
       (fmt.Println "original:" xs)
       (fmt.Println "empty:" (map square ^[int64] [])
                    (reduce add 10 ^[int64] []) (filter even ^[int64] [])))
  (let [offset 10]
       (fmt.Println "closure:" (map (fn ^int64 [^int64 x] (+ x offset)) [1 2 3]))))
