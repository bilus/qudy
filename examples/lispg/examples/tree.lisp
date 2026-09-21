;; Print a tree of the given directory, including the size of each file and directory.
(package main)
(import "fmt" "io" "io/fs" "log" "os" "path/filepath" "strings" "sync/atomic")

;; Buffer children so the directory header can include its final size.
;; io.Writer accepts both os.Stdout and the *strings.Builder created below.
(defn tree ^int64 [^string root ^string indent ^io.Writer output]
      (let [total (new atomic.Int64)
           children (new strings.Builder)
           childIndent (+ indent "  ")
           walkError
           (filepath.Walk
            root
            (fn ^error [^string path ^fs.FileInfo info ^error err]
                (if (not= err nil)
                    err
                    (if (= path root)
                        (if (info.IsDir) nil (fmt.Errorf "not a directory: %s" root))
                        (if (info.IsDir)
                            (let [size (tree path childIndent children)]
                                 (total.Add size)
                                 ;; tree has handled this subtree; do not visit it again.
                                 filepath.SkipDir)
                            (let [mode (info.Mode)]
                                 (if (mode.IsRegular)
                                     (let [size (info.Size)]
                                          (total.Add size)
                                          (fmt.Fprintf children "%s%s (%d B)\n" childIndent (info.Name) size))
                                     (fmt.Fprintf children "%s%s (not counted)\n" childIndent (info.Name)))
                                 nil))))))]
           (when (not= walkError nil) (log.Fatal walkError))
           (let [size (total.Load)
                name (filepath.Base root)
                ^string label (if (= name (string filepath.Separator)) name (+ name "/"))]
                (fmt.Fprintf output "%s%s (%d B)\n%s" indent label size (children.String))
                size)))

(defn main []
  (let [args (rest os.Args)
       ^string root (if (empty? args) "." (first args))]
       (tree (filepath.Clean root) "" os.Stdout)))
