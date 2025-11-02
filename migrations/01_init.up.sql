CREATE TABLE orders (
  id UUID PRIMARY KEY,
  customer_name TEXT NOT NULL,
  item TEXT NOT NULL,
  created_at TIMESTAMP WITH TIME ZONE DEFAULT now()
);