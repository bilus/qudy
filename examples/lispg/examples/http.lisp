(package main)
(import "fmt" "log" "net/http")

(defn main []
  (http.HandleFunc
   "/"
   (fn [^http.ResponseWriter w ^*http.Request r]
       (fmt.Fprintf w "Hello, %s\n" r.URL.Path)))

  (log.Fatal
   (http.ListenAndServe ":8080" nil)))
