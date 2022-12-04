;;; Directory Local Variables
;;; For more information see (info "(emacs) Directory Variables")

((nil (compile-command . "go build ./cmd/server"))
 (go-mode (eglot-workspace-configuration
	   (:gopls (matcher . "CaseInsensitive")
		   (staticcheck . t)
		   (gofumpt . t)))))
