package seed

import (
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/glvd/seed/model"
	shell "github.com/godcong/go-ipfs-restapi"
	"go.uber.org/atomic"
	"golang.org/x/xerrors"
)

// InfoFlag ...
type InfoFlag string

// InfoFlagNone ...
const InfoFlagNone InfoFlag = "none"

// InfoFlagInfo ...
const InfoFlagInfo InfoFlag = "info"

// InfoFlagUpdate ...
const InfoFlagUpdate InfoFlag = "update"

// InfoFlagMysql ...
const InfoFlagMysql InfoFlag = "mysql"

// InfoFlagJSON ...
const InfoFlagJSON InfoFlag = "json"

// InfoFlagBSON ...
const InfoFlagBSON InfoFlag = "bson"

// InfoFlagSQLite ...
const InfoFlagSQLite InfoFlag = "sqlite3"

// Information ...
type Information struct {
	seed      *Seed
	workspace string
	from      InfoFlag
	Path      string
	thread    int
	list      []string
	videos    map[string]*model.Video
	moves     map[string]string
	maxLimit  int
	noCheck   bool
}

// Option ...
func (info *Information) Option(seed *Seed) {
	informationOption(info)(seed)
}

// NewInformation ...
func NewInformation() *Information {
	return new(Information)
}

// BeforeRun ...
func (info *Information) BeforeRun(seed *Seed) {
	info.seed = seed
}

// AfterRun ...
func (info *Information) AfterRun(seed *Seed) {
}

func fixBson(s []byte) []byte {
	reg := regexp.MustCompile(`("_id")[ ]*[:][ ]*(ObjectId\(")[\w]{24}("\))[ ]*(,)[ ]*`)
	return reg.ReplaceAll(s, []byte(" "))
}

func video(source *VideoSource) (video *model.Video) {

	//always not null
	alias := *new([]string)
	aliasS := ""
	if source.Alias != nil && len(source.Alias) > 0 {
		alias = source.Alias
		aliasS = alias[0]
	}
	//always not null
	role := *new([]string)
	roleS := ""
	if source.Role != nil && len(source.Role) > 0 {
		role = source.Role
		roleS = role[0]
	}

	intro := source.Intro
	if intro == "" {
		intro = aliasS + " " + roleS
	}

	return &model.Video{
		FindNo:       strings.ToUpper(strings.ReplaceAll(strings.ReplaceAll(source.Bangumi, "-", ""), "_", "")),
		Bangumi:      strings.ToUpper(source.Bangumi),
		Type:         source.Type,
		Systematics:  source.Systematics,
		Sharpness:    source.Sharpness,
		Producer:     source.Producer,
		Language:     source.Language,
		Caption:      source.Caption,
		Intro:        intro,
		Alias:        alias,
		Role:         role,
		Director:     source.Director,
		Series:       source.Series,
		Tags:         source.Tags,
		Date:         source.Date,
		SourceHash:   source.SourceHash,
		Season:       MustString(source.Season, "1"),
		Episode:      MustString(source.Episode, "1"),
		TotalEpisode: MustString(source.TotalEpisode, "1"),
		Format:       MustString(source.Format, "2D"),
		Publisher:    source.Publisher,
		Length:       source.Length,
		MagnetLinks:  source.MagnetLinks,
		Uncensored:   source.Uncensored,
	}
}

// defaultUnfinished ...
func defaultUnfinished(name string) *model.Unfinished {
	_, file := filepath.Split(name)

	uncat := &model.Unfinished{
		Model:       model.Model{},
		Checksum:    "",
		Type:        "other",
		Relate:      "",
		Name:        file,
		Hash:        "",
		Sharpness:   "",
		Caption:     "",
		Encrypt:     false,
		Key:         "",
		M3U8:        "media.m3u8",
		SegmentFile: "media-%05d.ts",
		Sync:        false,
		Object:      new(model.VideoObject),
	}
	log.With("file", name).Info("calculate checksum")
	uncat.Checksum = model.Checksum(name)
	return uncat
}

func checkFileNotExist(path string) bool {
	_, e := os.Stat(path)
	if e != nil {
		if os.IsNotExist(e) {
			return true
		}
		return false
	}
	return false
}

// Run ...
func (info *Information) Run(ctx context.Context) {
	log.Info("Information running")
	var vs []*VideoSource
	select {
	case <-ctx.Done():
	default:
		switch info.from {
		case InfoFlagBSON:
			b, e := ioutil.ReadFile(info.Path)
			if e != nil {
				return
			}
			fixed := fixBson(b)
			reader := bytes.NewBuffer(fixed)
			e = LoadFrom(&vs, reader)
			if e != nil {
				log.Error(e)
				return
			}
		case InfoFlagJSON:
			b, e := ioutil.ReadFile(info.Path)
			if e != nil {
				return
			}
			reader := bytes.NewBuffer(b)
			e = LoadFrom(&vs, reader)
			if e != nil {
				log.Error(e)
				return
			}
		case InfoFlagSQLite:
			if info.list == nil {
				videos, e := model.AllVideos(nil, 0)
				if e != nil {
					log.Error(e)
					return
				}
				for _, video := range *videos {
					info.videos[video.Bangumi] = video
				}
				return
			}

			for _, name := range info.list {
				video, e := model.FindVideo(nil, name)
				if e != nil {
					log.Error(e)
					continue
				}
				info.videos[video.Bangumi] = video
			}
			//all work was done
			return
		}
	}
	log.With("total", len(vs)).Info("all Information")
	if vs == nil {
		log.Info("no videos to process")
		return
	}
	log.Info("list", info.list)
	vs = filterList(vs, info.list)
	max := len(vs)
	if max > info.maxLimit && info.maxLimit != 0 {
		max = info.maxLimit
	}
	log.With("size", len(vs), "max", max).Info("video source")
	skipIPFS := atomic.NewBool(false)
	v1 := make(chan *model.Video)
	moves := make(chan map[string]string)
	go func(v1 chan<- *model.Video, moves chan<- map[string]string) {
		runner := max
		m := make(map[string]string)
		defer func() {
			log.With("moves", m).Info("defer")
			moves <- m
		}()
		for i, s := range vs {
			if runner <= 0 {
				log.Info("break")
				return
			}
			v := video(s)
			var added bool
			if !skipIPFS.Load() {
				added = false
				if s.Poster != "" {
					v.PosterHash = s.Poster
				} else {
					if s.PosterPath != "" {
						s.PosterPath = filepath.Join(info.workspace, s.PosterPath)
						if checkFileNotExist(s.PosterPath) {
							log.With("run", runner, "index", i, "bangumi", s.Bangumi).Info("poster not found")
						} else {
							added = true
							poster, e := addPosterHash(info.seed, s)
							if e != nil {
								log.Error(e)
								skipIPFS.Store(true)
							} else {
								v.PosterHash = poster.Hash
								m[s.PosterPath] = poster.Hash
							}
						}
					}
				}

				if s.Thumb != "" {
					s.Thumb = filepath.Join(info.workspace, s.Thumb)
					if checkFileNotExist(s.Thumb) {
						log.With("run", runner, "index", i, "bangumi", s.Bangumi).Info("thumb not found")
					} else {
						added = true
						thumb, e := addThumbHash(info.seed, s)
						if e != nil {
							log.Error(e)
							skipIPFS.Store(true)
						} else {
							v.ThumbHash = thumb.Hash
							m[s.Thumb] = thumb.Hash
						}
					}
				}
			}
			if added || info.noCheck {
				log.With("run", runner, "index", i, "bangumi", s.Bangumi).Info("added")
				runner--
				v1 <- v
			}

		}
		for ; runner > 0; runner-- {
			v1 <- nil
		}
		log.Info("end")
	}(v1, moves)

	for ; max > 0; max-- {
		select {
		case v := <-v1:
			log.With("max", max, "data", v).Info("add video")
			if v == nil {
				continue
			}
			info.videos[v.Bangumi] = v
			e := model.AddOrUpdateVideo(v)
			if e != nil {
				log.Error(e)
				continue
			}
		}
	}

	info.moves = <-moves
	log.With("detail", info.moves).Info("move")
	return
}

func addThumbHash(seed *Seed, source *VideoSource) (*model.Unfinished, error) {
	unfinThumb := defaultUnfinished(source.Thumb)
	unfinThumb.Type = model.TypeThumb
	unfinThumb.Relate = source.Bangumi
	if source.Thumb != "" {
		abs, e := filepath.Abs(source.Thumb)
		if e != nil {
			return nil, e
		}

		object, e := shell.AddFile(abs)
		if e != nil {
			return nil, e
		}

		unfinThumb.Hash = object.Hash
		e = model.AddOrUpdateUnfinished(unfinThumb)
		if e != nil {
			return nil, e
		}
		return unfinThumb, nil
	}

	return nil, xerrors.New("no thumb")
}

func addPosterHash(shell *shell.Shell, source *VideoSource) (*model.Unfinished, error) {
	unfinPoster := defaultUnfinished(source.PosterPath)
	unfinPoster.Type = model.TypePoster
	unfinPoster.Relate = source.Bangumi

	if source.PosterPath != "" {
		abs, e := filepath.Abs(source.PosterPath)
		if e != nil {
			return nil, e
		}
		object, e := shell.AddFile(abs)
		if e != nil {
			return nil, e
		}
		unfinPoster.Hash = object.Hash
		e = model.AddOrUpdateUnfinished(unfinPoster)
		if e != nil {
			return nil, e
		}
		return unfinPoster, nil
	}
	return nil, xerrors.New("no poster")
}

func filterList(sources []*VideoSource, list []string) (vs []*VideoSource) {
	if list == nil || len(list) <= 0 {
		return sources
	}
	for _, source := range sources {
		for _, v := range list {
			if source.Bangumi == v {
				vs = append(vs, source)
			}
		}
	}
	return
}

// VideoSource ...
type VideoSource struct {
	Bangumi      string    `json:"bangumi"`       //番号 no
	SourceHash   string    `json:"source_hash"`   //原片hash
	Type         string    `json:"type"`          //类型：film，FanDrama
	Format       string    `json:"format"`        //输出：3D，2D
	VR           string    `json:"vr"`            //VR格式：左右，上下，平面
	Thumb        string    `json:"thumb"`         //缩略图
	Intro        string    `json:"intro"`         //简介 title
	Alias        []string  `json:"alias"`         //别名，片名
	VideoEncode  string    `json:"video_encode"`  //视频编码
	AudioEncode  string    `json:"audio_encode"`  //音频编码
	Files        []string  `json:"files"`         //存放路径
	HashFiles    []string  `json:"hash_files"`    //已上传Hash
	CheckFiles   []string  `json:"check_files"`   //Unfinished checksum
	Slice        bool      `json:"sliceAdd"`      //是否HLS切片
	Encrypt      bool      `json:"encrypt"`       //加密
	Key          string    `json:"key"`           //秘钥
	M3U8         string    `json:"m3u8"`          //M3U8名
	SegmentFile  string    `json:"segment_file"`  //ts切片名
	PosterPath   string    `json:"poster_path"`   //海报路径
	Poster       string    `json:"poster"`        //海报HASH
	ExtendList   []*Extend `json:"extend_list"`   //扩展信息
	Role         []string  `json:"role"`          //角色列表 stars
	Director     string    `json:"director"`      //导演
	Systematics  string    `json:"systematics"`   //分级
	Season       string    `json:"season"`        //季
	Episode      string    `json:"episode"`       //集数
	TotalEpisode string    `json:"total_episode"` //总集数
	Sharpness    string    `json:"sharpness"`     //清晰度
	Publish      string    `json:"publish"`       //发行日
	Date         string    `json:"date"`          //发行日
	Length       string    `json:"length"`        //片长
	Producer     string    `json:"producer"`      //制片商
	Series       string    `json:"series"`        //系列
	Tags         []string  `json:"tags"`          //标签
	Publisher    string    `json:"publisher"`     //发行商
	Language     string    `json:"language"`      //语言
	Caption      string    `json:"caption"`       //字幕
	MagnetLinks  []string  `json:"magnet_links"`  //磁链
	Uncensored   bool      `json:"uncensored"`    //有码,无码
}
