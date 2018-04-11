package metric

const (
	//PROC_STAT define proc file to calc cpu
	PROC_STAT = "/proc/stat"
	//PROC_DIR define proc files dir
	PROC_DIR = "/proc"

	//INDOCKER_PROC_STAT define mounted stat file in docker
	INDOCKER_PROC_STAT = "/hostProc/stat"
	//INDOCKER_PROC_DIR defin mounted proc dir in docker
	INDOCKER_PROC_DIR = "/hostProc"
)
