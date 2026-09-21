(package main)
(import "fmt")

(defn ^int64 maybe [^bool enabled]
  (when enabled
    (fmt.Print "body ")
    (let [x 20] (* x 2))))

(defn ^string label [^bool enabled] (when enabled "yes"))
(defn ^float64 amount [^bool enabled] (when enabled 1.5))
(defn ^bool active [^bool enabled] (when enabled true))

(defn ^bool condition [] (fmt.Println "condition") true)

(defn main []
  (fmt.Println (maybe true))
  (fmt.Println (maybe false))
  (fmt.Printf "%q %q\n" (label true) (label false))
  (fmt.Println (amount true) (amount false))
  (fmt.Println (active true) (active false))
  (fmt.Println ^int64 (let [x 40] (fmt.Print "let ") (+ x 2)))
  (let [^int64 value (when true 7)]
    (fmt.Println (+ value 1) (+ ^int64 (when true 10) 2)))
  (fmt.Println ^int64 (when true) ^int64 (when false)
    ^int64 (let []) ^int64 (if false 99))
  (fmt.Println ^int64 (let [x (int64 5)] (when (condition) x))))
