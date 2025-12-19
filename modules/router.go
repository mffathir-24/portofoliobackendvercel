package modules

import (
	"database/sql"
	"fmt"
	handlers "gintugas/modules/ServiceRoute"
	serviceroute "gintugas/modules/ServiceRoute"
	projectRPO "gintugas/modules/components/Project/repository"
	repositoryprojek "gintugas/modules/components/Project/repository"
	projectServsc "gintugas/modules/components/Project/service"
	"gintugas/modules/components/experiences/repo"
	"gintugas/modules/components/experiences/service"
	"gintugas/modules/utils"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	portfolioRepo "gintugas/modules/components/all/repo"
	portfolioService "gintugas/modules/components/all/service"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"gorm.io/gorm"
)

func Initiator(router *gin.Engine, db *sql.DB, gormDB *gorm.DB) {
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "Accept", "X-Requested-With"},
		ExposeHeaders:    []string{"Content-Length", "Content-Disposition"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// ⭐ CEK DATABASE STATUS
	dbAvailable := db != nil && gormDB != nil
	fmt.Printf("\n🔍 Database Status: available=%v\n", dbAvailable)

	uploadBasePath := getUploadPath()

	// ============================
	// STORAGE CONFIGURATION
	// ============================
	fmt.Println("=== STORAGE CONFIGURATION ===")

	supabaseURL := os.Getenv("SUPABASE_URL")
	supabaseServiceKey := os.Getenv("SUPABASE_SERVICE_ROLE_KEY")
	bucket := os.Getenv("SUPABASE_STORAGE_BUCKET")
	uploadProvider := os.Getenv("UPLOAD_PROVIDER")

	if supabaseServiceKey != "" && len(supabaseServiceKey) > 10 {
		fmt.Printf("SUPABASE_SERVICE_ROLE_KEY: %s\n",
			strings.Repeat("*", len(supabaseServiceKey)-10)+supabaseServiceKey[len(supabaseServiceKey)-10:])
	} else if supabaseServiceKey != "" {
		fmt.Printf("SUPABASE_SERVICE_ROLE_KEY: %s\n", strings.Repeat("*", len(supabaseServiceKey)))
	} else {
		fmt.Printf("SUPABASE_SERVICE_ROLE_KEY: (not set)\n")
	}

	var supabaseUploadService *utils.SupabaseUploadService

	if supabaseURL != "" && supabaseServiceKey != "" {
		fmt.Println("\n🔄 Initializing Supabase Storage...")
		supabaseUploadService = utils.NewSupabaseUploadService(supabaseURL, supabaseServiceKey, bucket)
		uploadProvider = "supabase"
		fmt.Println("✅ Supabase Storage initialized")
	} else {
		uploadProvider = "local"
		fmt.Println("\n⚠️  Supabase Storage not configured, using local storage")
	}

	// ============================
	// SWAGGER
	// ============================
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// ============================
	// API ROUTES
	// ============================
	api := router.Group("/api")
	{
		// Health check
		api.GET("/health", func(c *gin.Context) {
			c.JSON(200, gin.H{
				"status":    "ok",
				"service":   "gintugas-api",
				"timestamp": time.Now().Unix(),
				"upload":    uploadProvider,
				"database":  dbAvailable,
			})
		})

		// ⭐ CEK DATABASE SEBELUM INIT ROUTES
		if !dbAvailable {
			// Database tidak tersedia - hanya serve basic routes
			api.GET("/status", func(c *gin.Context) {
				c.JSON(503, gin.H{
					"error":   "Database not configured",
					"message": "Please set DATABASE_URL environment variable",
					"upload":  uploadProvider,
				})
			})

			fmt.Println("⚠️  Skipping database-dependent routes (database not available)")
			return
		}

		// ============================
		// INIT SERVICES (Hanya jika DB tersedia)
		// ============================
		fmt.Println("\n✅ Database available - initializing services...")

		// PROJECT SERVICES
		projectRepo := projectRPO.NewRepository(db)
		var projectService projectServsc.Service
		if uploadProvider == "supabase" && supabaseUploadService != nil {
			supabaseWrapper := projectServsc.NewSupabaseUploadWrapper(supabaseUploadService)
			projectService = projectServsc.NewServiceWithUpload(projectRepo, supabaseWrapper, "projects")
		} else {
			localPath := filepath.Join(uploadBasePath, "projects")
			projectService = projectServsc.NewService(projectRepo, localPath)
		}
		projectHandler := handlers.NewProjectHandler(projectService)

		memberRepo := repositoryprojek.NewProjectMemberRepo(gormDB)
		memberService := projectServsc.NewProjectMemberService(memberRepo, projectRepo)

		tagsrepo := projectRPO.NewTagsRepository(gormDB)
		tagsService := projectServsc.NewTaskService(tagsrepo)
		tagsHandler := handlers.NewTagsHandler(tagsService)

		// EXPERIENCE SERVICES
		expeRepo := repo.NewExpeGormRepository(gormDB)
		expeService := service.NewExpeService(expeRepo)
		expeHandler := serviceroute.NewGormExpeHandler(expeService)

		// PORTFOLIO SERVICES
		skillRepo := portfolioRepo.NewSkillRepository(gormDB)
		var skillService portfolioService.SkillService
		if uploadProvider == "supabase" && supabaseUploadService != nil {
			supabaseWrapper := portfolioService.NewSupabaseUploadWrapper(supabaseUploadService)
			skillService = portfolioService.NewSkillServiceWithUpload(skillRepo, supabaseWrapper, "skills")
		} else {
			localPath := filepath.Join(uploadBasePath, "skills")
			skillService = portfolioService.NewSkillService(skillRepo, localPath)
		}
		skillHandler := handlers.NewSkillHandler(skillService)

		certRepo := portfolioRepo.NewCertificateRepository(gormDB)
		var certService portfolioService.CertificateService
		if uploadProvider == "supabase" && supabaseUploadService != nil {
			supabaseWrapper := portfolioService.NewSupabaseUploadWrapper(supabaseUploadService)
			certService = portfolioService.NewCertificateServiceWithUpload(certRepo, supabaseWrapper, "certificates")
		} else {
			localPath := filepath.Join(uploadBasePath, "certificates")
			certService = portfolioService.NewCertificateService(certRepo, localPath)
		}
		certHandler := handlers.NewCertificateHandler(certService)

		eduRepo := portfolioRepo.NewEducationRepository(gormDB)
		eduService := portfolioService.NewEducationService(eduRepo)
		eduHandler := handlers.NewEducationHandler(eduService)

		testRepo := portfolioRepo.NewTestimonialRepository(gormDB)
		testService := portfolioService.NewTestimonialService(testRepo)
		testHandler := handlers.NewTestimonialHandler(testService)

		blogRepo := portfolioRepo.NewBlogRepository(gormDB)
		blogService := portfolioService.NewBlogService(blogRepo)
		blogHandler := handlers.NewBlogHandler(blogService)

		sectionRepo := portfolioRepo.NewSectionRepository(gormDB)
		sectionService := portfolioService.NewSectionService(sectionRepo)
		sectionHandler := handlers.NewSectionHandler(sectionService)

		socialLinkRepo := portfolioRepo.NewSocialLinkRepository(gormDB)
		socialLinkService := portfolioService.NewSocialLinkService(socialLinkRepo)
		socialLinkHandler := handlers.NewSocialLinkHandler(socialLinkService)

		settingRepo := portfolioRepo.NewSettingRepository(gormDB)
		settingService := portfolioService.NewSettingService(settingRepo)
		settingHandler := handlers.NewSettingHandler(settingService)

		// ============================
		// REGISTER ALL ROUTES
		// ============================

		// PROJECT ROUTES
		projectRoutes := api.Group("/v1/projects")
		{
			projectRoutes.GET("", projectHandler.GetAllProjects)
			projectRoutes.GET("/:id", projectHandler.GetProject)
			projectRoutes.POST("/with-image", projectHandler.CreateProjectWithImage)
			projectRoutes.PUT("/:id", projectHandler.UpdateProject)
			projectRoutes.DELETE("/:id", projectHandler.DeleteProject)
		}

		projects := api.Group("/projects")
		{
			projects.POST("/:project_id/tags", memberService.AddTag)
			projects.DELETE("/:project_id/tags/:tag_id", memberService.RemoveTag)
			projects.GET("/:project_id/tags", memberService.GetProjectTags)
		}

		tags := api.Group("/v1/tags")
		{
			tags.POST("", tagsHandler.CreateTags)
			tags.GET("", projectHandler.GetAllTags)
		}

		// EXPERIENCE ROUTES
		expeRoutes := api.Group("/v1")
		{
			expeRoutes.POST("/experiences/with-relations", expeHandler.CreateExperiencesWithRelations)
			expeRoutes.GET("/experiences/with-relations", expeHandler.GetAllExperiencesWithRelations)
			expeRoutes.GET("/experiences/with-relations/:id", expeHandler.GetExperiencesByIDWithRelations)
			expeRoutes.PUT("/experiences/with-relations/:id", expeHandler.UpdateExperiencesWithRelations)
			expeRoutes.DELETE("/experiences/with-relations/:id", expeHandler.DeleteExperiencesWithRelations)
		}

		// PORTFOLIO ROUTES
		v1 := api.Group("/v1")

		skills := v1.Group("/skills")
		{
			skills.POST("", skillHandler.Create)
			skills.POST("/with-icon", skillHandler.CreateWithIcon)
			skills.PUT("/:id/with-icon", skillHandler.UpdateWithIcon)
			skills.GET("", skillHandler.GetAll)
			skills.GET("/featured", skillHandler.GetFeatured)
			skills.GET("/category/:category", skillHandler.GetByCategory)
			skills.GET("/:id", skillHandler.GetByID)
			skills.PUT("/:id", skillHandler.Update)
			skills.DELETE("/:id", skillHandler.Delete)
		}

		certificates := v1.Group("/certificates")
		{
			certificates.POST("", certHandler.Create)
			certificates.POST("/with-image", certHandler.CreateWithImage)
			certificates.GET("", certHandler.GetAll)
			certificates.GET("/:id", certHandler.GetByID)
			certificates.PUT("/:id", certHandler.Update)
			certificates.DELETE("/:id", certHandler.Delete)
		}

		education := v1.Group("/education")
		{
			education.POST("", eduHandler.CreateWithAchievements)
			education.GET("", eduHandler.GetAllWithAchievements)
			education.GET("/:id", eduHandler.GetByIDWithAchievements)
			education.PUT("/:id", eduHandler.UpdateWithAchievements)
			education.DELETE("/:id", eduHandler.DeleteWithAchievements)
		}

		testimonials := v1.Group("/testimonials")
		{
			testimonials.POST("", testHandler.Create)
			testimonials.GET("", testHandler.GetAll)
			testimonials.GET("/featured", testHandler.GetFeatured)
			testimonials.GET("/status/:status", testHandler.GetByStatus)
			testimonials.GET("/:id", testHandler.GetByID)
			testimonials.PUT("/:id", testHandler.Update)
			testimonials.DELETE("/:id", testHandler.Delete)
		}

		blog := v1.Group("/blog")
		{
			blog.POST("", blogHandler.CreateWithTags)
			blog.GET("", blogHandler.GetAllWithTags)
			blog.GET("/published", blogHandler.GetPublishedWithTags)
			blog.GET("/tags", blogHandler.GetAllTags)
			blog.GET("/:id", blogHandler.GetByIDWithTags)
			blog.GET("/slug/:slug", blogHandler.GetBySlugWithTags)
			blog.PUT("/:id", blogHandler.UpdateWithTags)
			blog.DELETE("/:id", blogHandler.DeleteWithTags)
		}

		sections := v1.Group("/sections")
		{
			sections.POST("", sectionHandler.Create)
			sections.GET("", sectionHandler.GetAll)
			sections.DELETE("/:id", sectionHandler.Delete)
		}

		socialLinks := v1.Group("/social-links")
		{
			socialLinks.POST("", socialLinkHandler.Create)
			socialLinks.GET("", socialLinkHandler.GetAll)
			socialLinks.DELETE("/:id", socialLinkHandler.Delete)
		}

		settings := v1.Group("/settings")
		{
			settings.POST("", settingHandler.Create)
			settings.GET("", settingHandler.GetAll)
			settings.DELETE("/:id", settingHandler.Delete)
		}
	}

	// SERVE STATIC FILES (Development only)
	if os.Getenv("GIN_MODE") != "release" && uploadProvider == "local" {
		router.Static("/uploads", uploadBasePath)
		log.Printf("📁 Serving static files from: %s", uploadBasePath)
	} else {
		log.Printf("ℹ️  Using %s storage for uploads", uploadProvider)
	}
}

func getUploadPath() string {
	if os.Getenv("GIN_MODE") == "release" {
		if path := os.Getenv("UPLOAD_PATH"); path != "" {
			return path
		}
		return "/tmp/uploads"
	}
	return "./uploads"
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
