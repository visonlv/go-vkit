package utilsx

import "os"

//判断文件是否存在  存在返回 true 不存在返回false
func FileIsExist(filePath string) bool {
	var exist = true
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		exist = false
	}
	return exist
}
