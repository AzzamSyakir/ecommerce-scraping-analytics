CREATE TABLE product_category_trends (
      id uuid PRIMARY KEY NOT NULL,
      category_name VARCHAR(100) NOT NULL,
      sales_volume INT NOT NULL,
      trend_status VARCHAR(50) NOT NULL,
      analysis_date DATE NOT NULL,
      price_average NUMERIC(10, 2)
)