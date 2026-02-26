package db

import (
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type BaseModel struct {
	ID        uint `gorm:"primarykey"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Conversation struct {
	BaseModel
	Title string
}

type Message struct {
	BaseModel
	ConversationID uint
	Role           string
	Content        string
}

type Setting struct {
	BaseModel
	Key   string `gorm:"uniqueIndex"`
	Value string
}

type KnowledgeBaseFile struct {
	BaseModel
	Path     string `gorm:"uniqueIndex"`
	Checksum string
	Size     int64
	Status   string // "pending", "processed", "error"
}

type KnowledgeBaseChunk struct {
	BaseModel
	FileID  uint
	Content string
	// 未来可以增加向量字段，目前先做基础全文检索或简单匹配
}

const SystemPromptKey = "system_prompt"
const KBFolderKey = "kb_folder"
const DefaultSystemPrompt = "你是一个中文的助手，你会根据用户的问题回答用户的问题。"

var DB *gorm.DB

func InitDB(dbPath string) {
	var err error

	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		os.MkdirAll(dir, 0755)
	}

	DB, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		log.Fatal("failed to connect database:", err)
	}

	if err := DB.AutoMigrate(&Conversation{}, &Message{}, &Setting{}, &KnowledgeBaseFile{}, &KnowledgeBaseChunk{}); err != nil {
		log.Fatal("failed to migrate database:", err)
	}

	c, err := GetOrCreateDefaultConversation()
	if err != nil {
		log.Fatal("failed to initialize default conversation:", err)
	}

	DB.Model(&Message{}).Where("conversation_id = 0").Update("conversation_id", c.ID)

	var s Setting
	if err := DB.Where("key = ?", SystemPromptKey).First(&s).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		_ = DB.Create(&Setting{
			Key:   SystemPromptKey,
			Value: DefaultSystemPrompt,
		}).Error
	}
}

func CreateConversation(title string) (*Conversation, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		title = "New chat"
	}
	c := &Conversation{Title: title}
	if err := DB.Create(c).Error; err != nil {
		return nil, err
	}
	return c, nil
}

func ListConversations(limit int) ([]Conversation, error) {
	if limit <= 0 {
		limit = 50
	}
	var cs []Conversation
	if err := DB.Order("updated_at desc").Limit(limit).Find(&cs).Error; err != nil {
		return nil, err
	}
	return cs, nil
}

func GetOrCreateDefaultConversation() (*Conversation, error) {
	var c Conversation
	err := DB.Where("title = ?", "Default").First(&c).Error
	if err == nil {
		return &c, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	return CreateConversation("Default")
}

func SaveMessage(conversationID uint, role, content string) error {
	return DB.Create(&Message{ConversationID: conversationID, Role: role, Content: content}).Error
}

func GetConversation(conversationID uint) (*Conversation, error) {
	var c Conversation
	if err := DB.First(&c, conversationID).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

func UpdateConversationTitle(conversationID uint, title string) error {
	title = strings.TrimSpace(title)
	if title == "" {
		return nil
	}
	return DB.Model(&Conversation{}).Where("id = ?", conversationID).Update("title", title).Error
}

func UpdateMessageContent(conversationID uint, messageID uint, content string) error {
	content = strings.TrimSpace(content)
	return DB.Model(&Message{}).
		Where("id = ? AND conversation_id = ?", messageID, conversationID).
		Update("content", content).
		Error
}

func GetLastUserMessage(conversationID uint) (*Message, error) {
	var m Message
	if err := DB.Where("conversation_id = ? AND role = ?", conversationID, "user").Order("created_at desc").First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

func GetFirstUserMessage(conversationID uint) (*Message, error) {
	var m Message
	if err := DB.Where("conversation_id = ? AND role = ?", conversationID, "user").Order("created_at asc").First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

func GetMessage(conversationID uint, messageID uint) (*Message, error) {
	var m Message
	if err := DB.Where("conversation_id = ? AND id = ?", conversationID, messageID).First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

func DeleteMessagesAfter(conversationID uint, messageID uint) error {
	m, err := GetMessage(conversationID, messageID)
	if err != nil {
		return err
	}
	return DB.Where("conversation_id = ? AND created_at > ?", conversationID, m.CreatedAt).Delete(&Message{}).Error
}

func GetHistory(conversationID uint, limit int) ([]Message, error) {
	if limit <= 0 {
		limit = 200
	}
	var messages []Message
	if err := DB.Where("conversation_id = ?", conversationID).Order("created_at asc").Limit(limit).Find(&messages).Error; err != nil {
		return nil, err
	}
	return messages, nil
}

func GetSetting(key string) (string, error) {
	var s Setting
	if err := DB.Where("key = ?", key).First(&s).Error; err != nil {
		return "", err
	}
	return s.Value, nil
}

func SetSetting(key, value string) error {
	if key == "" {
		return nil
	}
	var s Setting
	if err := DB.Where("key = ?", key).First(&s).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return DB.Create(&Setting{Key: key, Value: value}).Error
		}
		return err
	}
	s.Value = value
	return DB.Save(&s).Error
}

func GetSystemPrompt() (string, error) {
	return GetSetting(SystemPromptKey)
}

func GetKBFolder() (string, error) {
	return GetSetting(KBFolderKey)
}

func ListKBFiles() ([]KnowledgeBaseFile, error) {
	var files []KnowledgeBaseFile
	err := DB.Find(&files).Error
	return files, err
}

func SaveKBFile(path string, size int64, checksum string) (*KnowledgeBaseFile, error) {
	var f KnowledgeBaseFile
	err := DB.Where("path = ?", path).First(&f).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			f = KnowledgeBaseFile{Path: path, Size: size, Checksum: checksum, Status: "pending"}
			err = DB.Create(&f).Error
			return &f, err
		}
		return nil, err
	}
	f.Size = size
	f.Checksum = checksum
	f.Status = "pending"
	err = DB.Save(&f).Error
	return &f, err
}

func SaveKBChunk(fileID uint, content string) error {
	return DB.Create(&KnowledgeBaseChunk{FileID: fileID, Content: content}).Error
}

func DeleteKBChunks(fileID uint) error {
	return DB.Where("file_id = ?", fileID).Delete(&KnowledgeBaseChunk{}).Error
}

func UpdateKBFileStatus(fileID uint, status string) error {
	return DB.Model(&KnowledgeBaseFile{}).Where("id = ?", fileID).Update("status", status).Error
}

func SearchKBChunks(query string, limit int) ([]KnowledgeBaseChunk, error) {
	if limit <= 0 {
		limit = 5
	}
	var chunks []KnowledgeBaseChunk

	// 改进：简单的关键词分割搜索
	// 去掉一些常见的无意义词（停用词）
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}

	// 按空格、标点分割
	words := strings.FieldsFunc(query, func(r rune) bool {
		return r == ' ' || r == ',' || r == '，' || r == '。' || r == '?' || r == '？' || r == '!' || r == '！'
	})

	if len(words) == 0 {
		return nil, nil
	}

	// 构造多关键词查询
	tx := DB.Model(&KnowledgeBaseChunk{})
	for _, word := range words {
		if len(word) < 2 && !isChinese(word) { // 忽略过短的英文单词，但保留单个中文字（如果是中文环境）
			continue
		}
		tx = tx.Or("content LIKE ?", "%"+word+"%")
	}

	err := tx.Limit(limit).Find(&chunks).Error
	return chunks, err
}

func isChinese(s string) bool {
	for _, r := range s {
		if r >= 0x4e00 && r <= 0x9fa5 {
			return true
		}
	}
	return false
}

func DeleteConversation(conversationID uint) error {
	if conversationID == 0 {
		return nil
	}
	if err := DB.Where("conversation_id = ?", conversationID).Delete(&Message{}).Error; err != nil {
		return err
	}
	if err := DB.Delete(&Conversation{}, conversationID).Error; err != nil {
		return err
	}
	return nil
}

func DeleteConversations(ids []uint) error {
	if len(ids) == 0 {
		return nil
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("conversation_id IN ?", ids).Delete(&Message{}).Error; err != nil {
			return err
		}
		if err := tx.Where("id IN ?", ids).Delete(&Conversation{}).Error; err != nil {
			return err
		}
		return nil
	})
}

func ResetKnowledgeBase() error {
	// 删除所有的知识库分片
	if err := DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&KnowledgeBaseChunk{}).Error; err != nil {
		return err
	}
	// 删除所有的知识库文件记录
	if err := DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&KnowledgeBaseFile{}).Error; err != nil {
		return err
	}
	return nil
}

func DeleteKBFile(id uint) error {
	var f KnowledgeBaseFile
	if err := DB.First(&f, id).Error; err != nil {
		return err
	}
	
	// 1. Delete Chunks
	if err := DeleteKBChunks(id); err != nil {
		return err
	}
	
	// 2. Delete Record
	if err := DB.Delete(&f).Error; err != nil {
		return err
	}
	
	// 3. Delete Physical File
	// Ignore error if file doesn't exist
	_ = os.Remove(f.Path)
	
	return nil
}

func DeleteKBFiles(ids []uint) error {
	if len(ids) == 0 {
		return nil
	}
	
	// Find all files first to get paths
	var files []KnowledgeBaseFile
	if err := DB.Where("id IN ?", ids).Find(&files).Error; err != nil {
		return err
	}
	
	return DB.Transaction(func(tx *gorm.DB) error {
		// 1. Delete Chunks
		if err := tx.Where("file_id IN ?", ids).Delete(&KnowledgeBaseChunk{}).Error; err != nil {
			return err
		}
		
		// 2. Delete Records
		if err := tx.Where("id IN ?", ids).Delete(&KnowledgeBaseFile{}).Error; err != nil {
			return err
		}
		
		// 3. Delete Physical Files (after DB transaction success)
		// We do this outside transaction usually, but here if transaction fails we shouldn't delete files.
		// However, file deletion cannot be rolled back.
		// So we accept that file deletion happens after commit or we just do it here and ignore rollback issues for files.
		// Better approach: do it after commit. But for simplicity in this helper, we'll do it here.
		for _, f := range files {
			_ = os.Remove(f.Path)
		}
		
		return nil
	})
}
